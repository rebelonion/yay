package upgrade

import (
	"context"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

func UpDevel(
	ctx context.Context,
	log *text.Logger,
	remote map[string]db.IPackage,
	aurdata map[string]*query.Pkg,
	localCache vcs.Store,
) UpSlice {
	toRemove := make([]string, 0)
	toUpgrade := UpSlice{Up: make([]Upgrade, 0), Repos: []string{"devel"}}

	for pkgName, pkg := range remote {
		if localCache.ToUpgrade(ctx, pkgName) {
			if _, ok := aurdata[pkgName]; !ok {
				log.Warnln(gotext.Get("ignoring package devel upgrade (no AUR info found):"), pkgName)
				continue
			}

			if pkg.ShouldIgnore() {
				printIgnoringPackage(log, pkg, "latest-commit")
				continue
			}

			toUpgrade.Up = append(toUpgrade.Up,
				Upgrade{
					Name:          pkg.Name(),
					Base:          pkg.Base(),
					Repository:    "devel",
					LocalVersion:  pkg.Version(),
					RemoteVersion: "latest-commit",
					Reason:        pkg.Reason(),
				})
		}
	}

	localCache.RemovePackages(toRemove)

	return toUpgrade
}

func printIgnoringPackage(log *text.Logger, pkg db.IPackage, newPkgVersion string) {
	left, right := query.GetVersionDiff(pkg.Version(), newPkgVersion)

	pkgName := pkg.Name()
	log.Warnln(gotext.Get("%s: ignoring package upgrade (%s => %s)",
		text.Cyan(pkgName),
		left, right,
	))
}

// UpAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list. tooNew contains packages skipped due to minAge.
func UpAUR(log *text.Logger, remote map[string]db.IPackage, aurdata map[string]*query.Pkg,
	enableDowngrade bool, minAge time.Duration,
) (toUpgrade, tooNew UpSlice) {
	toUpgrade = UpSlice{Up: make([]Upgrade, 0), Repos: []string{"aur"}}
	tooNew = UpSlice{Up: make([]Upgrade, 0), Repos: []string{"aur"}}

	for name, pkg := range remote {
		aurPkg, ok := aurdata[name]
		if !ok {
			continue
		}

		if (db.VerCmp(pkg.Version(), aurPkg.Version) < 0) ||
			(enableDowngrade && (db.VerCmp(pkg.Version(), aurPkg.Version) > 0)) {
			if pkg.ShouldIgnore() {
				printIgnoringPackage(log, pkg, aurPkg.Version)
				continue
			}

			if minAge > 0 {
				age := time.Since(time.Unix(int64(aurPkg.LastModified), 0))
				if age < minAge {
					tooNew.Up = append(tooNew.Up, Upgrade{
						Name:          aurPkg.Name,
						Base:          aurPkg.PackageBase,
						Repository:    "aur",
						LocalVersion:  pkg.Version(),
						RemoteVersion: aurPkg.Version,
						Reason:        pkg.Reason(),
						Extra:         text.FormatDuration(age),
					})

					continue
				}
			}

			toUpgrade.Up = append(toUpgrade.Up,
				Upgrade{
					Name:          aurPkg.Name,
					Base:          aurPkg.PackageBase,
					Repository:    "aur",
					LocalVersion:  pkg.Version(),
					RemoteVersion: aurPkg.Version,
					Reason:        pkg.Reason(),
				})
		}
	}

	return toUpgrade, tooNew
}
