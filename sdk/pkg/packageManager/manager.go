package packageManager

// // Generic PackageManager
// type PackageManager[T any, S PackageSource[T]] struct {
// 	source S
// }

// // Constructor
// func NewPackageManager[T any, S PackageSource[T]](source S) *PackageManager[T, S] {
// 	return &PackageManager[T, S]{source: source}
// }

// // Load method
// func (pm *PackageManager[T, S]) Load(ctx context.Context) (*ApplicationPackage, error) {
// 	if err := pm.source.Validate(); err != nil {
// 		return nil, fmt.Errorf("source validation failed: %w", err)
// 	}
// 	return pm.source.Load(ctx)
// }
