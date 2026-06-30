package api

import (
	"io"
	"net/http/httptest"
	"testing"

	"api.audius.co/config"
	"api.audius.co/rendezvous"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func withContentAssetConfig(t *testing.T, storeAll []string, blacklisted []string, dead []string, nodes []config.Node) {
	t.Helper()

	oldStoreAll := config.Cfg.StoreAllNodes
	oldBlacklisted := config.Cfg.BlacklistedNodes
	oldDead := config.Cfg.DeadNodes
	oldHasher := rendezvous.GlobalHasher

	config.Cfg.StoreAllNodes = storeAll
	config.Cfg.BlacklistedNodes = blacklisted
	config.Cfg.DeadNodes = dead
	rendezvous.Refresh(nodes)

	t.Cleanup(func() {
		config.Cfg.StoreAllNodes = oldStoreAll
		config.Cfg.BlacklistedNodes = oldBlacklisted
		config.Cfg.DeadNodes = oldDead
		rendezvous.GlobalHasher = oldHasher
	})
}

func TestContentAssetRedirectUsesStoreAllNode(t *testing.T) {
	app := contentAssetTestApp()
	withContentAssetConfig(t,
		[]string{"https://store-all.test", "https://blocked.test"},
		[]string{"https://blocked.test"},
		nil,
		[]config.Node{{Endpoint: "https://validator.test"}},
	)

	req := httptest.NewRequest("GET", "/content/test-cid/150x150.jpg?cache=bust", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)

	require.Equal(t, 307, res.StatusCode)
	require.Equal(t, "https://store-all.test/content/test-cid/150x150.jpg?cache=bust", res.Header.Get("Location"))
	require.Equal(t, "https://store-all.test", res.Header.Get(contentAssetUpstreamHeader))
}

func TestContentAssetRedirectSupportsLegacyContentPath(t *testing.T) {
	app := contentAssetTestApp()
	withContentAssetConfig(t,
		[]string{"https://store-all.test"},
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest("GET", "/content/test-cid", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)

	require.Equal(t, 307, res.StatusCode)
	require.Equal(t, "https://store-all.test/content/test-cid", res.Header.Get("Location"))
}

func TestContentAssetRedirectFallsBackToRendezvous(t *testing.T) {
	app := contentAssetTestApp()
	withContentAssetConfig(t,
		nil,
		nil,
		nil,
		[]config.Node{{Endpoint: "https://validator.test"}},
	)

	req := httptest.NewRequest("GET", "/content/test-cid/150x150.jpg", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)

	require.Equal(t, 307, res.StatusCode)
	require.Equal(t, "https://validator.test/content/test-cid/150x150.jpg", res.Header.Get("Location"))
}

func TestContentAssetRedirectNoCandidates(t *testing.T) {
	app := contentAssetTestApp()
	withContentAssetConfig(t, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/content/test-cid/150x150.jpg", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)

	require.Equal(t, 503, res.StatusCode)
}

func TestContentVerboseKeepsNodeListRoute(t *testing.T) {
	app := contentAssetTestApp()
	withContentAssetConfig(t,
		[]string{"https://store-all.test"},
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest("GET", "/content/verbose", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	require.Equal(t, 200, res.StatusCode)
	require.JSONEq(t, `{"data":null}`, string(body))
}

func contentAssetTestApp() *ApiServer {
	app := &ApiServer{
		App:        fiber.New(),
		logger:     zap.NewNop(),
		validators: NewNodes(),
	}
	app.Get("/content", app.contentNodes)
	app.Get("/content/verbose", app.contentNodes)
	app.Get("/content/:cid", app.contentAssetRedirect)
	app.Get("/content/:cid/:asset", app.contentAssetRedirect)
	return app
}
