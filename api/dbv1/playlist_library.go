package dbv1

import (
	"encoding/json"
	"fmt"

	"bridgerton.audius.co/trashid"
)

// PlaylistLibraryItem represents a generic item in the playlist library
type PlaylistLibraryItem interface{}

// PlaylistLibrary represents the root structure of the playlist library
type PlaylistLibrary struct {
	Contents []PlaylistLibraryItem `json:"contents"`
}

// RegularPlaylist represents a standard playlist in the library
type RegularPlaylist struct {
	Type       string         `json:"type"`
	PlaylistID trashid.HashId `json:"playlist_id"`
}

// ExplorePlaylist represents an explore playlist in the library
type ExplorePlaylist struct {
	Type       string `json:"type"`
	PlaylistID string `json:"playlist_id"`
}

// Folder represents a folder containing playlist items
type Folder struct {
	Type     string                `json:"type"`
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Contents []PlaylistLibraryItem `json:"contents"`
}

func (f *Folder) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type     string            `json:"type"`
		ID       string            `json:"id"`
		Name     string            `json:"name"`
		Contents []json.RawMessage `json:"contents"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Type = raw.Type
	f.ID = raw.ID
	f.Name = raw.Name
	f.Contents = make([]PlaylistLibraryItem, 0, len(raw.Contents))
	for _, item := range raw.Contents {
		var itemType string
		if err := json.Unmarshal(item, &itemType); err != nil {
			return err
		}
		switch itemType {
		case "playlist":
			var playlist RegularPlaylist
			if err := json.Unmarshal(item, &playlist); err != nil {
				return err
			}
			f.Contents = append(f.Contents, playlist)
		case "explore_playlist":
			var explorePlaylist ExplorePlaylist
			if err := json.Unmarshal(item, &explorePlaylist); err != nil {
				return err
			}
			f.Contents = append(f.Contents, explorePlaylist)
		case "folder":
			var folder Folder
			if err := json.Unmarshal(item, &folder); err != nil {
				return err
			}
			f.Contents = append(f.Contents, folder)
		default:
			return fmt.Errorf("unknown item type: %s", itemType)
		}
	}
	return nil
}

func (pl *PlaylistLibrary) UnmarshalJSON(data []byte) error {
	type RawLibrary struct {
		Contents []json.RawMessage `json:"contents"`
	}

	var rawLibrary RawLibrary
	if err := json.Unmarshal(data, &rawLibrary); err != nil {
		return err
	}

	pl.Contents = make([]PlaylistLibraryItem, 0, len(rawLibrary.Contents))

	for _, item := range rawLibrary.Contents {
		// First, determine the type of item
		var typeWrapper struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(item, &typeWrapper); err != nil {
			return err
		}

		// Parse based on the type
		switch typeWrapper.Type {
		case "playlist":
			var playlist RegularPlaylist
			if err := json.Unmarshal(item, &playlist); err != nil {
				return err
			}
			pl.Contents = append(pl.Contents, playlist)
		case "explore_playlist":
			var explorePlaylist ExplorePlaylist
			if err := json.Unmarshal(item, &explorePlaylist); err != nil {
				return err
			}
			pl.Contents = append(pl.Contents, explorePlaylist)
		case "folder":
			type RawFolder struct {
				Contents []json.RawMessage `json:"contents"`
			}
			var rawFolder RawFolder
			if err := json.Unmarshal(item, &rawFolder); err != nil {
				return err
			}
			var folder Folder
			if err := json.Unmarshal(item, &folder); err != nil {
				return err
			}
			pl.Contents = append(pl.Contents, folder)
		default:
			return fmt.Errorf("unknown item type: %s", typeWrapper.Type)
		}
	}

	return nil
}
