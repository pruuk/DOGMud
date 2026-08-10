package characters

import (
	"os"
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

func AltsExists(userId int) bool {
	_, err := os.Stat(util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/users/`, strconv.Itoa(userId)+`.alts.yaml`))

	return !os.IsNotExist(err)
}

func LoadAlts(userId int) []Character {

	if !AltsExists(userId) {
		return nil
	}

	altsFilePath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/users/`, strconv.Itoa(userId)+`.alts.yaml`)

	altsFileBytes, err := os.ReadFile(altsFilePath)
	if err != nil {
		mudlog.Error("LoadAlts", "error", err.Error())
		return nil
	}

	altsRecords := []Character{}

	if err := yaml.Unmarshal(altsFileBytes, &altsRecords); err != nil {
		mudlog.Error("LoadAlts", "error", err.Error())
	}

	return altsRecords

}

func SaveAlts(userId int, alts []Character) bool {

	fileWritten := false
	tmpSaved := false
	tmpCopied := false
	completed := false

	defer func() {
		mudlog.Info("SaveAlts()", "userId", strconv.Itoa(userId), "wrote-file", fileWritten, "tmp-file", tmpSaved, "tmp-copied", tmpCopied, "completed", completed)
	}()

	data, err := yaml.Marshal(&alts)
	if err != nil {
		mudlog.Error("SaveAlts", "error", err.Error())
		return false
	}

	carefulSave := configs.GetFilePathsConfig().CarefulSaveFiles

	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/users/`, strconv.Itoa(userId)+`.alts.yaml`)

	// Routed through util.Save (chunk 2.8): same hand-rolled careful save as
	// SaveUser had — atomic but not durable — and alt characters are player
	// data too. Also drops mode 0777; a character file has no business being
	// executable.
	if err := util.Save(path, data, bool(carefulSave)); err != nil {
		mudlog.Error("SaveAlts", "error", err.Error())
		return false
	}
	fileWritten = true
	if carefulSave {
		tmpSaved = true
		tmpCopied = true
	}

	completed = true

	return true

}
