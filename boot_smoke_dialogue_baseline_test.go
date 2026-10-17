package main

// knownDialogueValidatorFindings is the accepted baseline of editor-validator
// errors in shipped dialogue, keyed by the exact line
// TestSmoke_ShippedDialoguePassesTheEditorValidator reports. It is a drift
// gate, not an approval: a NEW finding fails CI, and these wait for an owner
// call because several rules may misfire on step-advance grant nodes (a node
// that grants "N-end" to finish a quest is not an offer, so the "quest"/"task"
// ask-word rule and the grant-first ordering rule may not apply to it), and
// the missing tokens ("17-letter", "11-sylara", "11-rhett") are gates on
// branches no player can reach. Recorded 2026-10-17 when the sweep first ran.
//
// The semicolon rule is never baselined: a ';' in spoken text truncates it in
// play, so every one is fixed in the content instead.
//
// To clear an entry: fix the content (or the rule), confirm the line no longer
// reports, and delete it here.
var knownDialogueValidatorFindings = map[string]bool{
	"_datafiles/world/dogmud/dialogue/amber_valley/9397.yaml: node \"water_mediate\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":               true,
	"_datafiles/world/dogmud/dialogue/ashwick/259.yaml: node \"apprentice_deep\": quest token \"17-letter\" does not exist":                                                                                           true,
	"_datafiles/world/dogmud/dialogue/ashwick/259.yaml: node \"apprentice_surface\": quest token \"17-letter\" does not exist":                                                                                        true,
	"_datafiles/world/dogmud/dialogue/ashwick/259.yaml: node \"quest_complete_forager\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":            true,
	"_datafiles/world/dogmud/dialogue/ashwick/259.yaml: node \"quest_complete_herbs\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":              true,
	"_datafiles/world/dogmud/dialogue/ashwick/259.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                                true,
	"_datafiles/world/dogmud/dialogue/ashwick/260.yaml: node \"family_deep\": quest token \"17-letter\" does not exist":                                                                                               true,
	"_datafiles/world/dogmud/dialogue/ashwick/260.yaml: node \"family_surface\": quest token \"17-letter\" does not exist":                                                                                            true,
	"_datafiles/world/dogmud/dialogue/ashwick/260.yaml: node \"trouble_deep\": quest token \"17-letter\" does not exist":                                                                                              true,
	"_datafiles/world/dogmud/dialogue/ashwick/260.yaml: node \"trouble_surface\": quest token \"17-letter\" does not exist":                                                                                           true,
	"_datafiles/world/dogmud/dialogue/ashwick/262.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                                true,
	"_datafiles/world/dogmud/dialogue/dustwalk_road/83.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                           true,
	"_datafiles/world/dogmud/dialogue/greenford/9516.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                             true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/240.yaml: node \"the_problem\": quest token \"11-rhett\" does not exist":                                                                                        true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/240.yaml: node \"the_problem\": quest token \"11-sylara\" does not exist":                                                                                       true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/240.yaml: root variant 1: quest token \"11-rhett\" does not exist":                                                                                              true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/240.yaml: root variant 1: quest token \"11-sylara\" does not exist":                                                                                             true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/240.yaml: root variant 2: quest token \"11-sylara\" does not exist":                                                                                             true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/240.yaml: root variant 3: quest token \"11-rhett\" does not exist":                                                                                              true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/241.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/ironwind_steppe/242.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/labyrinth_of_low_tunnels/74.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                true,
	"_datafiles/world/dogmud/dialogue/marches_spur_road/250.yaml: node \"quest_complete_dialogue\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works": true,
	"_datafiles/world/dogmud/dialogue/marches_spur_road/250.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                      true,
	"_datafiles/world/dogmud/dialogue/new_plymouth_common/9320.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                   true,
	"_datafiles/world/dogmud/dialogue/new_plymouth_docks/9300.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                    true,
	"_datafiles/world/dogmud/dialogue/new_plymouth_merchant/9349.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                 true,
	"_datafiles/world/dogmud/dialogue/new_plymouth_temple/9360.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                   true,
	"_datafiles/world/dogmud/dialogue/north_road/277.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                             true,
	"_datafiles/world/dogmud/dialogue/north_road/281.yaml: node \"hand_over\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":                      true,
	"_datafiles/world/dogmud/dialogue/north_road/281.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                             true,
	"_datafiles/world/dogmud/dialogue/north_road/286.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                             true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9100.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9112.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9117.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9129.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9137.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9145.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9161.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/pothole_coulee/9162.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/stillwater/342.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                             true,
	"_datafiles/world/dogmud/dialogue/stillwater/347.yaml: node \"ask_journal\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":                    true,
	"_datafiles/world/dogmud/dialogue/stillwater/355.yaml: node \"ask_kingfisher\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":                 true,
	"_datafiles/world/dogmud/dialogue/stillwater/355.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                             true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9465.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_aegis\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":               true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_bandolier\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":           true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_blackrazor\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":          true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_choir\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":               true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_phial\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":               true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_prism\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":               true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_thornwall\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":           true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_wayfarer\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":            true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: node \"teach_zephyr\": quest-granting entries must include \"quest\" and \"task\" in their triggers/keywords so `ask <npc> quest` works":              true,
	"_datafiles/world/dogmud/dialogue/the_confluence/9584.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                        true,
	"_datafiles/world/dogmud/dialogue/thornwall_city/94.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                          true,
	"_datafiles/world/dogmud/dialogue/thornwall_city/95.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                          true,
	"_datafiles/world/dogmud/dialogue/thornwall_city/96.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                          true,
	"_datafiles/world/dogmud/dialogue/thornwall_outskirts/89.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                     true,
	"_datafiles/world/dogmud/dialogue/watchers_crossing/84.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                       true,
	"_datafiles/world/dogmud/dialogue/watchers_crossing/86.yaml: tree.nodes: quest-grant nodes must come FIRST":                                                                                                       true,
}
