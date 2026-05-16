//go:build !integration
// +build !integration

package dep

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Jguer/aur"
	"github.com/Jguer/dyalpm"
	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestGrapher_GraphFromTargetsBranches(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(string) mock.IPackage { return nil },
		SatisfierFromDBFn: func(_, db string) (mock.IPackage, error) {
			return &mock.Package{
				PName:    "repo-pkg",
				PVersion: "1",
				PDB:      mock.NewDB(db),
			}, nil
		},
		PackagesFromGroupAndDBFn: func(name, dbName string) ([]mock.IPackage, error) {
			return nil, nil
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{}, false, false, false, false, false, 0, logger)
	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"core/repo-pkg"})
	require.NoError(t, err)
	require.True(t, graph.Exists("repo-pkg"))
}

func TestGrapher_GraphFromTargetsFallbackToGroup(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		SyncSatisfierFn: func(string) mock.IPackage {
			return nil
		},
		PackagesFromGroupFn: func(name string) []mock.IPackage {
			return []mock.IPackage{
				&mock.Package{
					PName: "grouped",
					PDB:   mock.NewDB("extra"),
				},
			}
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{}, false, false, false, false, false, 0, logger)
	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"grouped"})
	require.NoError(t, err)
	require.True(t, graph.Exists("grouped"))
	require.True(t, graph.GetNodeInfo("grouped").Value.IsGroup)
}

func TestGrapher_GraphFromTargetsFallbackToAur(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		SyncSatisfierFn:     func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalPackageFn:      func(string) mock.IPackage { return nil },
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{Name: "aur-target", Version: "1"}}, nil
		},
	}, false, false, false, false, false, 0, logger)

	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"aur-target"})
	require.NoError(t, err)
	require.Equal(t, AUR, graph.GetNodeInfo("aur-target").Value.Source)
}

func TestGrapher_GraphFromAURNeededSkipsUpToDate(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(name string) mock.IPackage {
			return &mock.Package{
				PName:    name,
				PVersion: "2",
				PReason:  dyalpm.PkgReasonExplicit,
			}
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{
				Name:    "needless",
				Version: "1",
			}}, nil
		},
	}, false, false, false, false, true, 0, logger)

	graph, err := grapher.GraphFromAUR(context.Background(), nil, []string{"needless"})
	require.NoError(t, err)
	require.False(t, graph.Exists("needless"))
}

func TestGrapher_GraphFromAURNotFoundReturnsErr(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	grapher := NewGrapher(&mock.DBExecutor{}, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return nil, nil
		},
	}, false, false, false, false, false, 0, logger)

	graph, err := grapher.GraphFromAUR(context.Background(), nil, []string{"ghost"})
	var targetNotFound *query.ErrTargetNotFound
	require.ErrorAs(t, err, &targetNotFound)
	require.NotNil(t, graph)
	require.False(t, graph.Exists("ghost"))
}

func TestGrapher_GraphFromSrcInfosSinglePkg(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(string) mock.IPackage { return nil },
		AlpmArchitecturesFn: func() ([]string, error) {
			return []string{"x86_64"}, nil
		},
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{}, false, false, false, false, false, 0, logger)
	graph, err := grapher.GraphFromSrcInfos(context.Background(), nil, map[string]*gosrc.Srcinfo{
		"foo": {
			PackageBase: gosrc.PackageBase{
				Pkgbase: "foo",
				Pkgver:  "1",
			},
			Packages: []gosrc.Package{
				{
					Pkgname: "foo",
				},
			},
		},
	})

	require.NoError(t, err)
	info := graph.GetNodeInfo("foo")
	require.NotNil(t, info)
	require.Equal(t, SrcInfo, info.Value.Source)
}

func TestGrapher_FindDepsFromAURSatisfiesMissingAndErrorPaths(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalSatisfierExistsFn: func(string) bool { return false },
		SyncSatisfierFn:        func(string) mock.IPackage { return nil },
	}

	grapher := NewGrapher(dbExecutor, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name:    "dep-provider",
					Version: "2.0",
					Provides: []string{
						"dep-lib",
					},
				},
			}, nil
		},
	}, false, false, false, false, false, 0, logger)

	graph := NewGraph()
	graph.AddNode("root")
	graph.AddProvides("dep-lib", &dyalpm.Depend{Name: "dep-lib", Version: "2.0"}, "dep-provider")
	deps := mapset.NewThreadUnsafeSet("dep-lib")
	found := grapher.findDepsFromAUR(context.Background(), graph, "root", deps)
	require.Len(t, found, 1)
}

func TestConfirmTooNewPkgs(t *testing.T) {
	t.Parallel()

	recentPkg := aur.Pkg{
		Name:         "new-pkg",
		PackageBase:  "new-pkg",
		Version:      "1.0",
		LastModified: int(time.Now().Add(-1 * time.Hour).Unix()),
	}

	mockDB := &mock.DBExecutor{
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}
	mockAUR := &mockaur.MockAUR{
		GetFn: func(_ context.Context, q *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{recentPkg}, nil
		},
	}

	t.Run("no-op when no packages collected", func(t *testing.T) {
		t.Parallel()

		logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
		grapher := NewGrapher(mockDB, mockAUR, false, false, false, false, false, 0, logger)
		require.NoError(t, grapher.ConfirmTooNewPkgs())
	})

	t.Run("warns and proceeds when user confirms", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		logger := text.NewLogger(&stdout, io.Discard, strings.NewReader("y\n"), false, "test")
		grapher := NewGrapher(mockDB, mockAUR, false, false, false, false, false, 7*24*time.Hour, logger)

		_, err := grapher.GraphFromAUR(context.Background(), nil, []string{"new-pkg"})
		require.NoError(t, err)

		require.NoError(t, grapher.ConfirmTooNewPkgs())
		require.Contains(t, stdout.String(), "new-pkg")
	})

	t.Run("returns ErrUserAbort when user declines", func(t *testing.T) {
		t.Parallel()

		logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader("n\n"), false, "test")
		grapher := NewGrapher(mockDB, mockAUR, false, false, false, false, false, 7*24*time.Hour, logger)

		_, err := grapher.GraphFromAUR(context.Background(), nil, []string{"new-pkg"})
		require.NoError(t, err)

		var abort *settings.ErrUserAbort
		require.ErrorAs(t, grapher.ConfirmTooNewPkgs(), &abort)
	})

	t.Run("clears state after call so second call is no-op", func(t *testing.T) {
		t.Parallel()

		logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader("y\n"), false, "test")
		grapher := NewGrapher(mockDB, mockAUR, false, false, false, false, false, 7*24*time.Hour, logger)

		_, err := grapher.GraphFromAUR(context.Background(), nil, []string{"new-pkg"})
		require.NoError(t, err)

		require.NoError(t, grapher.ConfirmTooNewPkgs())
		require.NoError(t, grapher.ConfirmTooNewPkgs())
	})
}
