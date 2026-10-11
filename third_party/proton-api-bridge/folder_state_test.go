package proton_api_bridge

import (
	"errors"
	"strings"
	"testing"

	"github.com/rclone/go-proton-api"
)

func TestRequireActiveFolderFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		link *proton.Link
		want error
	}{
		{name: "missing link", want: ErrLinkMustNotBeNil},
		{name: "trashed directory", link: &proton.Link{Type: proton.LinkTypeFolder, State: proton.LinkStateTrashed}, want: ErrLinkMustBeActive},
		{name: "active file", link: &proton.Link{Type: proton.LinkTypeFile, State: proton.LinkStateActive}, want: ErrLinkTypeMustToBeFolderType},
		{name: "active directory", link: &proton.Link{Type: proton.LinkTypeFolder, State: proton.LinkStateActive}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireActiveFolder(tc.link)
			if !errors.Is(err, tc.want) {
				t.Fatalf("requireActiveFolder() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDistinctDirectoryNamesRejectAuthenticatedCollisions(t *testing.T) {
	tests := []struct {
		name    string
		entries []*ProtonDirectoryData
		wantErr bool
	}{
		{name: "empty folder"},
		{
			name: "different names",
			entries: []*ProtonDirectoryData{
				{Name: "one", Link: &proton.Link{LinkID: "a"}},
				{Name: "two", Link: &proton.Link{LinkID: "b"}},
			},
		},
		{
			name: "duplicate file name",
			entries: []*ProtonDirectoryData{
				{Name: "same", Link: &proton.Link{LinkID: "a"}},
				{Name: "same", Link: &proton.Link{LinkID: "b"}},
			},
			wantErr: true,
		},
		{
			name: "file and folder collision",
			entries: []*ProtonDirectoryData{
				{Name: "same", Link: &proton.Link{LinkID: "a"}},
				{Name: "same", Link: &proton.Link{LinkID: "b"}, IsFolder: true},
			},
			wantErr: true,
		},
		{
			name: "repeated link",
			entries: []*ProtonDirectoryData{
				{Name: "same", Link: &proton.Link{LinkID: "a"}},
				{Name: "same", Link: &proton.Link{LinkID: "a"}},
			},
			wantErr: true,
		},
		{name: "empty authenticated filename", entries: []*ProtonDirectoryData{
			{Name: "", Link: &proton.Link{LinkID: "a"}},
		}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireDistinctDirectoryNames(tc.entries)
			if (err != nil) != tc.wantErr {
				t.Fatalf("requireDistinctDirectoryNames() = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(err.Error(), "directory") {
				t.Fatalf("failure should identify incomplete directory listing: %v", err)
			}
		})
	}
}
