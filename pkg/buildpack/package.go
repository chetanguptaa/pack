package buildpack

import (
	"io"

	"github.com/pkg/errors"

	"github.com/buildpacks/pack/internal/style"
	"github.com/buildpacks/pack/pkg/dist"
)

type Package interface {
	Label(name string) (value string, err error)
	GetLayer(diffID string) (io.ReadCloser, error)
}

func ExtractBuildpacks(pkg Package) (mainBP BuildModule, depBPs []BuildModule, err error) {
	md := &Metadata{}
	if found, err := dist.GetLabel(pkg, MetadataLabel, md); err != nil {
		return nil, nil, err
	} else if !found {
		return nil, nil, errors.Errorf(
			"could not find label %s",
			style.Symbol(MetadataLabel),
		)
	}

	pkgLayers := dist.ModuleLayers{}
	ok, err := dist.GetLabel(pkg, dist.BuildpackLayersLabel, &pkgLayers)
	if err != nil {
		return nil, nil, err
	}

	if !ok {
		return nil, nil, errors.Errorf(
			"could not find label %s",
			style.Symbol(dist.BuildpackLayersLabel),
		)
	}

	for bpID, v := range pkgLayers {
		for bpVersion, bpInfo := range v {
			desc := dist.BuildpackDescriptor{
				API: bpInfo.API,
				Info: dist.ModuleInfo{
					ID:       bpID,
					Version:  bpVersion,
					Homepage: bpInfo.Homepage,
					Name:     bpInfo.Name,
				},
				Stacks: bpInfo.Stacks,
				Order:  bpInfo.Order,
			}

			diffID := bpInfo.LayerDiffID // Allow use in closure
			b := &openerBlob{
				opener: func() (io.ReadCloser, error) {
					rc, err := pkg.GetLayer(diffID)
					if err != nil {
						return nil, errors.Wrapf(err,
							"extracting buildpack %s layer (diffID %s)",
							style.Symbol(desc.Info.FullName()),
							style.Symbol(diffID),
						)
					}
					return rc, nil
				},
			}

			if desc.Info.Match(md.ModuleInfo) { // This is the order buildpack of the package
				mainBP = FromBlob(&desc, b)
			} else {
				depBPs = append(depBPs, FromBlob(&desc, b))
			}
		}
	}

	return mainBP, depBPs, nil
}

func ExtractExtensions(pkg Package) (mainBP BuildModule, err error) {
	pkgLayers := dist.ModuleLayers{}
	ok, err := dist.GetLabel(pkg, dist.BuildpackLayersLabel, &pkgLayers)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.Errorf(
			"could not find label %s",
			style.Symbol(dist.BuildpackLayersLabel),
		)
	}

	for extID, v := range pkgLayers {
		for extVersion, extInfo := range v {
			desc := dist.ExtensionDescriptor{
				API: extInfo.API,
				Info: dist.ModuleInfo{
					ID:       extID,
					Version:  extVersion,
					Homepage: extInfo.Homepage,
					Name:     extInfo.Name,
				},
			}

			diffID := extInfo.LayerDiffID // Allow use in closure
			b := &openerBlob{
				opener: func() (io.ReadCloser, error) {
					rc, err := pkg.GetLayer(diffID)
					if err != nil {
						return nil, errors.Wrapf(err,
							"extracting extension %s layer (diffID %s)",
							style.Symbol(desc.Info.FullName()),
							style.Symbol(diffID),
						)
					}
					return rc, nil
				},
			}

			mainBP = FromBlob(&desc, b)
		}
	}

	return mainBP, nil
}

type openerBlob struct {
	opener func() (io.ReadCloser, error)
}

func (b *openerBlob) Open() (io.ReadCloser, error) {
	return b.opener()
}
