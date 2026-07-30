// Package releaseinfo defines the one release identity emitted by a build.
package releaseinfo

const Version = "v0.1.2"

func ArchiveName(extension string) string {
	return "araihu-assets-" + Version + "." + extension
}
