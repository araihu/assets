// Package releaseinfo defines the one release identity emitted by a build.
package releaseinfo

const Version = "v0.2.0"

func ArchiveName(extension string) string {
	return "araihu-assets-" + Version + "." + extension
}
