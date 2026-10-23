package api

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"api.audius.co/api/dbv1"
	apiutils "api.audius.co/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	stemsZipMaxPerPod  = 32
	stemsZipMaxPerUser = 2

	// A content node that stops sending bytes for this long is treated as
	// failed, and the file resumes from the next mirror.
	stemsZipStallTimeout = 30 * time.Second
)

var stemsZipSlots = newDownloadSlots(stemsZipMaxPerPod, stemsZipMaxPerUser)

// No overall timeout: a large stem legitimately takes minutes. Stalls are
// caught per read instead, see copyWithStallTimeout.
var stemsZipClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   8,
	},
}

type stemsZipEntry struct {
	name string
	size int64
	link *dbv1.MediaLink
}

// v1TrackStemsDownload streams a track's stems, and optionally the parent, as
// one zip. Entries are stored uncompressed so the archive's length is known
// before any audio is fetched, which lets the browser's download manager show
// real progress.
func (app *ApiServer) v1TrackStemsDownload(c *fiber.Ctx) error {
	trackId := int32(c.Locals("trackId").(int))
	includeParent := c.QueryBool("include_parent")

	rootName, entries, err := app.stemsZipEntries(c, trackId, includeParent)
	if err != nil {
		return err
	}

	slotKey := apiutils.GetIP(c)
	if myId := app.getMyId(c); myId != 0 {
		slotKey = strconv.Itoa(int(myId))
	}
	if !stemsZipSlots.acquire(slotKey) {
		return fiber.NewError(fiber.StatusTooManyRequests, "too many downloads in progress")
	}

	headers := stemsZipHeaders(rootName, entries, time.Now())
	length, err := stemsZipLength(headers)
	if err != nil {
		stemsZipSlots.release(slotKey)
		return err
	}

	logger := app.logger.With(zap.Int32("track_id", trackId), zap.Int64("bytes", length))
	pr, pw := io.Pipe()
	go func() {
		defer stemsZipSlots.release(slotKey)
		err := writeStemsZip(pw, headers, entries)
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			logger.Warn("stems zip download failed", zap.Error(err))
		}
		pw.CloseWithError(err)
	}()

	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, attachmentDisposition(rootName+".zip"))
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	// Keep ingress-nginx from spooling the whole zip to disk for slow clients.
	c.Set("X-Accel-Buffering", "no")
	// fasthttp closes pr once the response ends, successful or not, which
	// fails the next write and stops the goroutine.
	c.Context().SetBodyStream(pr, int(length))
	return nil
}

// stemsZipEntries resolves the files the requester may download, with a
// signed link and size for each. Files that can't be downloaded are left out
// rather than failing the whole archive.
func (app *ApiServer) stemsZipEntries(c *fiber.Ctx, trackId int32, includeParent bool) (string, []stemsZipEntry, error) {
	rows, err := app.pool.Query(c.Context(), `
	SELECT s.child_track_id
	FROM stems s
	JOIN tracks t ON t.track_id = s.child_track_id
	WHERE s.parent_track_id = @track_id
	  AND t.is_current = true
	  AND t.is_delete = false
	ORDER BY s.child_track_id
	`, pgx.NamedArgs{"track_id": trackId})
	if err != nil {
		return "", nil, err
	}
	stemIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return "", nil, err
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID:            app.getMyId(c),
			Ids:             append([]int32{trackId}, stemIds...),
			AuthedWallet:    app.tryGetAuthedWallet(c),
			IncludeUnlisted: true,
		},
	})
	if err != nil {
		return "", nil, err
	}

	byId := make(map[int32]dbv1.Track, len(tracks))
	for _, t := range tracks {
		byId[t.TrackID] = t
	}

	parent, ok := byId[trackId]
	if !ok || !parent.IsAudioAllowed {
		return "", nil, fiber.NewError(fiber.StatusNotFound, "track not found")
	}
	if len(stemIds) == 0 {
		return "", nil, fiber.NewError(fiber.StatusNotFound, "track has no stems")
	}

	var candidates []dbv1.Track
	if includeParent {
		candidates = append(candidates, parent)
	}
	for _, id := range stemIds {
		if t, ok := byId[id]; ok && t.IsAudioAllowed {
			candidates = append(candidates, t)
		}
	}
	type resolved struct {
		track       dbv1.Track
		link        *dbv1.MediaLink
		size        int64
		contentType string
	}
	var files []*resolved
	denied := false
	for _, t := range candidates {
		link := t.Download
		if link == nil {
			link, err = app.ownerDownloadLink(c, &t)
			if err != nil {
				return "", nil, err
			}
		}
		if link == nil {
			denied = denied || !t.Access.Download
			continue
		}
		files = append(files, &resolved{track: t, link: link})
	}

	g := errgroup.Group{}
	for _, f := range files {
		g.Go(func() error {
			info, err := inspectCid(f.track.DownloadCid())
			if err != nil {
				app.logger.Warn("stems zip: skipping file with no blob info",
					zap.Int32("track_id", f.track.TrackID), zap.Error(err))
				return nil
			}
			f.size = info.Size
			f.contentType = info.ContentType
			return nil
		})
	}
	g.Wait()

	rootName := sanitizeZipName(parent.Title.String)
	if rootName == "" {
		rootName = "stems"
	}

	seen := map[string]int{}
	var entries []stemsZipEntry
	for _, f := range files {
		if f.size <= 0 {
			continue
		}
		entries = append(entries, stemsZipEntry{
			name: dedupeZipName(stemsZipFilename(&f.track, f.contentType), seen),
			size: f.size,
			link: f.link,
		})
	}

	if len(entries) == 0 {
		if denied {
			return "", nil, fiber.NewError(fiber.StatusForbidden, "you are not allowed to download these stems")
		}
		return "", nil, fiber.NewError(fiber.StatusNotFound, "no downloadable files")
	}
	return rootName, entries, nil
}

var audioExtensions = map[string]string{
	"audio/wav":    ".wav",
	"audio/wave":   ".wav",
	"audio/x-wav":  ".wav",
	"audio/flac":   ".flac",
	"audio/x-flac": ".flac",
	"audio/aiff":   ".aiff",
	"audio/x-aiff": ".aiff",
	"audio/mpeg":   ".mp3",
	"audio/mp4":    ".m4a",
	"audio/x-m4a":  ".m4a",
	"audio/ogg":    ".ogg",
}

// stemsZipFilename is the download filename, except that an original with no
// recorded filename gets its extension from the blob's content type rather
// than createFilename's .mp3, since the bytes are the original upload.
func stemsZipFilename(t *dbv1.Track, contentType string) string {
	name := sanitizeZipName(createFilename(t))
	if t.OrigFileCid.String != "" && t.OrigFilename.String == "" {
		if ext, ok := audioExtensions[strings.ToLower(contentType)]; ok {
			name = strings.TrimSuffix(name, path.Ext(name)) + ext
		}
	}
	if name == "" || strings.HasPrefix(name, ".") {
		name = "track" + name
	}
	return name
}

func sanitizeZipName(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\':
			return '_'
		case r < 0x20 || r == 0x7f:
			return -1
		}
		return r
	}, name)
	return strings.Trim(strings.TrimSpace(name), ".")
}

// dedupeZipName appends " (n)" before the extension when a name repeats,
// ignoring case so the zip extracts cleanly on macOS and Windows.
func dedupeZipName(name string, seen map[string]int) string {
	key := strings.ToLower(name)
	n := seen[key]
	seen[key] = n + 1
	if n == 0 {
		return name
	}
	ext := path.Ext(name)
	candidate := fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
	return dedupeZipName(candidate, seen)
}

func attachmentDisposition(filename string) string {
	fallback := strings.Map(func(r rune) rune {
		if r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, url.PathEscape(filename))
}

// stemsZipHeaders builds stored entries with a data descriptor. The CRC is
// filled in after each file is written, which archive/zip reads from the
// header pointer when it writes the descriptor and central directory.
func stemsZipHeaders(rootName string, entries []stemsZipEntry, modified time.Time) []*zip.FileHeader {
	date, tm := msDosTime(modified)
	headers := make([]*zip.FileHeader, len(entries))
	for i, e := range entries {
		name := rootName + "/" + e.name
		h := &zip.FileHeader{
			Name:               name,
			Method:             zip.Store,
			Flags:              0x8,
			CreatorVersion:     3<<8 | 20,
			ReaderVersion:      20,
			ExternalAttrs:      0o644 << 16,
			ModifiedDate:       date,
			ModifiedTime:       tm,
			CompressedSize64:   uint64(e.size),
			UncompressedSize64: uint64(e.size),
		}
		if !isASCII(name) {
			h.Flags |= 0x800
		}
		if e.size >= 1<<32-1 {
			h.ReaderVersion = 45
		}
		headers[i] = h
	}
	return headers
}

// stemsZipLength lays the archive out against a writer that only counts, so
// it shares every byte of framing with writeStemsZip, including the zip64
// records that depend on offsets past 4GiB.
func stemsZipLength(headers []*zip.FileHeader) (int64, error) {
	cw := &countingWriter{}
	zw := zip.NewWriter(cw)
	zeros := make([]byte, 1<<20)
	for _, h := range headers {
		hc := *h
		fw, err := zw.CreateRaw(&hc)
		if err != nil {
			return 0, err
		}
		for left := int64(h.UncompressedSize64); left > 0; {
			n := min(left, int64(len(zeros)))
			if _, err := fw.Write(zeros[:n]); err != nil {
				return 0, err
			}
			left -= n
		}
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	return cw.n, nil
}

func writeStemsZip(w io.Writer, headers []*zip.FileHeader, entries []stemsZipEntry) error {
	zw := zip.NewWriter(w)
	for i, h := range headers {
		fw, err := zw.CreateRaw(h)
		if err != nil {
			return err
		}
		sum := crc32.NewIEEE()
		if err := fetchStem(io.MultiWriter(fw, sum), entries[i]); err != nil {
			return fmt.Errorf("%s: %w", entries[i].name, err)
		}
		h.CRC32 = sum.Sum32()
	}
	return zw.Close()
}

// fetchStem writes exactly e.size bytes of the file, resuming with a Range
// request on the next mirror when a node fails partway through.
func fetchStem(w io.Writer, e stemsZipEntry) error {
	urls, err := mediaLinkUrls(e.link)
	if err != nil {
		return err
	}

	var written int64
	var lastErr error
	for _, u := range urls {
		n, err := fetchRange(w, u, written, e.size)
		written += n
		if err == nil {
			return nil
		}
		var we *writeError
		if errors.As(err, &we) {
			return we.err
		}
		lastErr = err
	}
	return fmt.Errorf("all mirrors failed after %d of %d bytes: %w", written, e.size, lastErr)
}

type writeError struct{ err error }

func (e *writeError) Error() string { return e.err.Error() }

func fetchRange(w io.Writer, u *url.URL, offset, size int64) (int64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return 0, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := stemsZipClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	switch {
	case offset == 0 && resp.StatusCode == http.StatusOK:
	case offset > 0 && resp.StatusCode == http.StatusPartialContent:
	default:
		return 0, fmt.Errorf("%s returned %d", u.Host, resp.StatusCode)
	}

	remaining := size - offset
	n, err := copyWithStallTimeout(w, io.LimitReader(resp.Body, remaining), cancel)
	if err != nil {
		return n, err
	}
	if n != remaining {
		return n, fmt.Errorf("%s ended after %d of %d bytes", u.Host, offset+n, size)
	}
	return n, nil
}

// copyWithStallTimeout copies like io.Copy but cancels the request when no
// bytes arrive for stemsZipStallTimeout. Write failures come back as
// *writeError so the caller can tell a gone client from a bad node.
func copyWithStallTimeout(w io.Writer, r io.Reader, cancel context.CancelFunc) (int64, error) {
	timer := time.AfterFunc(stemsZipStallTimeout, cancel)
	defer timer.Stop()

	buf := make([]byte, 64*1024)
	var total int64
	for {
		n, err := r.Read(buf)
		if n > 0 {
			timer.Reset(stemsZipStallTimeout)
			if _, werr := w.Write(buf[:n]); werr != nil {
				return total, &writeError{werr}
			}
			total += int64(n)
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// mediaLinkUrls lists the primary URL then each mirror with the same signed
// path, the order tryFindWorkingUrl uses.
func mediaLinkUrls(link *dbv1.MediaLink) ([]*url.URL, error) {
	main, err := url.Parse(link.Url)
	if err != nil {
		return nil, err
	}
	q := main.Query()
	q.Set("skip_play_count", "true")
	main.RawQuery = q.Encode()

	urls := []*url.URL{main}
	for _, mirror := range link.Mirrors {
		m, err := url.Parse(mirror)
		if err != nil || m.Host == main.Host {
			continue
		}
		u := *main
		u.Scheme = m.Scheme
		u.Host = m.Host
		urls = append(urls, &u)
	}
	return urls, nil
}

func msDosTime(t time.Time) (uint16, uint16) {
	t = t.UTC()
	if t.Year() < 1980 {
		t = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	date := uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	tm := uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return date, tm
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

type countingWriter struct{ n int64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

// downloadSlots caps concurrent long-lived downloads per pod and per key.
type downloadSlots struct {
	mu         sync.Mutex
	total      int
	perKey     map[string]int
	maxTotal   int
	maxPerUser int
}

func newDownloadSlots(maxTotal, maxPerUser int) *downloadSlots {
	return &downloadSlots{perKey: map[string]int{}, maxTotal: maxTotal, maxPerUser: maxPerUser}
}

func (s *downloadSlots) acquire(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.total >= s.maxTotal || s.perKey[key] >= s.maxPerUser {
		return false
	}
	s.total++
	s.perKey[key]++
	return true
}

func (s *downloadSlots) release(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total--
	if s.perKey[key] <= 1 {
		delete(s.perKey, key)
	} else {
		s.perKey[key]--
	}
}
