package rooms

import (
	"github.com/GoMudEngine/GoMud/internal/util"
)

func GetMemoryUsage() map[string]util.MemoryResult {

	ret := map[string]util.MemoryResult{}

	ret["rooms"] = util.MemoryResult{Memory: util.MemoryUsage(roomManager.rooms), Count: len(roomManager.rooms)}
	ret["zones"] = util.MemoryResult{Memory: util.MemoryUsage(roomManager.zones), Count: len(roomManager.zones)}
	ret["roomsWithUsers"] = util.MemoryResult{Memory: util.MemoryUsage(roomManager.roomsWithUsers), Count: len(roomManager.roomsWithUsers)}
	fileCache, fileCacheCt := roomManager.cachedFilePathStats()
	ret["roomIdToFileCache"] = util.MemoryResult{Memory: util.MemoryUsage(fileCache), Count: fileCacheCt}

	return ret
}

func init() {
	util.AddMemoryReporter(`Rooms`, GetMemoryUsage)
}
