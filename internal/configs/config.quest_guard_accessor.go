package configs

// QuestGuardLimits returns the quest engine's chain depth limit
// (Balance.QuestChainDepthLimit) and slow-run warning threshold in
// milliseconds (Balance.QuestPerformanceWarnMs).
//
// It exists for a measured reason, as GetLightingConfig does. The engine
// reads these two knobs on every event a player raises, room_enter on every
// step, and GetBalanceConfig() copies the whole Balance struct to hand them
// back: in BenchmarkNotifyRoomEnter (internal/questengine, shipped quests)
// that copy was about 130 of 320 ns per call. The knobs stay declared on
// Balance; only the read path is narrowed (#375 phase 3b slice 4).
func QuestGuardLimits() (maxDepth int, warnMs int64) {
	ensureConfigValidated()

	configDataLock.RLock()
	defer configDataLock.RUnlock()

	return int(configData.Balance.QuestChainDepthLimit), int64(configData.Balance.QuestPerformanceWarnMs)
}
