package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDBTotalSize(t *testing.T) {
	type fileSpec struct {
		name string
		data []byte
	}
	testCases := []struct {
		name        string
		files       []fileSpec // main file is always first, then optional -wal and -shm
		setup       func(paths []string) error
		expectSize  int64
		expectError bool
	}{
		{
			name: "MainOnly",
			files: []fileSpec{
				{name: "main", data: []byte("main-db-content")},
			},
			setup: func(paths []string) error {
				return os.WriteFile(paths[0], []byte("main-db-content"), 0644)
			},
			expectSize:  int64(len("main-db-content")),
			expectError: false,
		},
		{
			name: "WithWALAndSHM",
			files: []fileSpec{
				{name: "main", data: []byte("main-db")},
				{name: "wal", data: []byte("wal-content")},
				{name: "shm", data: []byte("shm-content")},
			},
			setup: func(paths []string) error {
				if err := os.WriteFile(paths[0], []byte("main-db"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(paths[1], []byte("wal-content"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(paths[2], []byte("shm-content"), 0644); err != nil {
					return err
				}
				return nil
			},
			expectSize:  int64(len("main-db") + len("wal-content") + len("shm-content")),
			expectError: false,
		},
		{
			name:        "MissingFiles",
			files:       []fileSpec{{name: "main", data: nil}},
			setup:       func(paths []string) error { return nil }, // don't create file
			expectSize:  0,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare temp files
			tmpDir := t.TempDir()
			mainPath := tmpDir + "/main.db"
			paths := []string{mainPath}
			if len(tc.files) > 1 {
				for _, f := range tc.files[1:] {
					paths = append(paths, mainPath+"-"+f.name)
				}
			}
			// Setup files as needed
			require.NoError(t, tc.setup(paths))
			// No permission cleanup needed for StatError (file is removed)
			// Remove files after test
			defer func() {
				for _, p := range paths {
					os.Remove(p)
				}
			}()

			size, err := DBTotalSize(mainPath)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectSize, size)
			}
		})
	}
}
