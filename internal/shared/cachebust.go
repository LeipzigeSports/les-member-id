package shared

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
)

// FileHashMap is a map of paths to the crc32 checksum of their contents.
type FileHashMap map[string]string

// InjectHashParameter looks up files from the static directory and runs a crc32 checksum on their contents.
// Once computed, the checksum is written to fileHashMap with the file path as its key. If a checksum for
// a file passed to this function is already present in fileHashMap, its stored value is used instead.
// The checksum is then appended as a URL path parameter to the asset path. This enables cache busting
// when file contents change.
func InjectHashParameter(fileHashMap FileHashMap, staticAssetPath string) string {
	hash, ok := fileHashMap[staticAssetPath]
	if ok {
		return appendHashParameter(staticAssetPath, hash)
	}

	bytes, err := os.ReadFile(filepath.Clean(staticAssetPath))
	if err != nil {
		panic(err)
	}

	crcSum := crc32.Checksum(bytes, crc32.IEEETable)

	return appendHashParameter(staticAssetPath, strconv.FormatUint(uint64(crcSum), 16))
}

func appendHashParameter(staticAssetPath, hash string) string {
	return fmt.Sprintf("%s?c=%s", staticAssetPath, hash)
}
