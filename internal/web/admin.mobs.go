package web

import (
	"net/http"
	"sort"
	"strconv"
	"text/template"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
)

func mobsIndex(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.New("index.html").Funcs(funcMap).ParseFiles(configs.GetFilePathsConfig().AdminHtml.String()+"/_header.html", configs.GetFilePathsConfig().AdminHtml.String()+"/mobs/index.html", configs.GetFilePathsConfig().AdminHtml.String()+"/_footer.html")
	if err != nil {
		mudlog.Error("HTML Template", "error", err)
	}

	allMobs := mobs.GetAllMobInfo()
	sort.SliceStable(allMobs, func(i, j int) bool {
		return allMobs[i].MobId < allMobs[j].MobId
	})

	mobIndexData := struct {
		Mobs []mobs.Mob
	}{
		allMobs,
	}

	if err := tmpl.Execute(w, mobIndexData); err != nil {
		mudlog.Error("HTML Execute", "error", err)
	}

}

func mobData(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.New("mob.data.html").Funcs(funcMap).ParseFiles(configs.GetFilePathsConfig().AdminHtml.String() + "/mobs/mob.data.html")
	if err != nil {
		mudlog.Error("HTML Template", "error", err)
	}

	urlVals := r.URL.Query()

	mobIdInt, _ := strconv.Atoi(urlVals.Get(`mobid`))

	mobInfo := mobs.GetMobSpec(mobs.MobId(mobIdInt))
	if mobInfo == nil {
		mobInfo = &mobs.Mob{}
	}

	mobGroupSet := map[string]struct{}{}
	allMobGroups := []string{}
	for _, m := range mobs.GetAllMobInfo() {

		for _, groupName := range m.Groups {
			if _, ok := mobGroupSet[groupName]; !ok {
				allMobGroups = append(allMobGroups, groupName)
				mobGroupSet[groupName] = struct{}{}
			}
		}

	}

	allSpecies := species.GetAllSpecies()
	sort.SliceStable(allSpecies, func(i, j int) bool {
		return allSpecies[i].SpeciesId < allSpecies[j].SpeciesId
	})

	allZoneNames := rooms.GetAllZoneNames()
	sort.SliceStable(allZoneNames, func(i, j int) bool {
		return allZoneNames[i] < allZoneNames[j]
	})

	activityLevels := []int{}
	for i := 1; i < 101; i++ {
		activityLevels = append(activityLevels, i)
	}

	dropChances := []int{}
	for i := 0; i < 101; i++ {
		dropChances = append(dropChances, i)
	}

	conditionSpecs := []conditions.ConditionSpec{}
	for _, conditionId := range conditions.GetAllConditionIds() {
		if b := conditions.GetConditionSpec(conditionId); b != nil {
			if b.Name == `empty` {
				continue
			}
			conditionSpecs = append(conditionSpecs, *b)
		}
	}
	sort.SliceStable(conditionSpecs, func(i, j int) bool {
		return conditionSpecs[i].ConditionId < conditionSpecs[j].ConditionId
	})

	tplData := map[string]any{}

	tplData[`mobInfo`] = *mobInfo

	shopData := map[string]characters.Shop{
		`Items`:       {},
		`Conditions`:  {},
		`Mercenaries`: {},
		`Pets`:        {},
	}

	for _, shopItm := range mobInfo.Character.Shop {

		if shopItm.ItemId > 0 {
			shopData[`Items`] = append(shopData[`Items`], shopItm)
			continue
		}

		if shopItm.ConditionId > 0 {
			shopData[`Conditions`] = append(shopData[`Conditions`], shopItm)
			continue
		}

		if shopItm.MobId > 0 {
			shopData[`Mercenaries`] = append(shopData[`Mercenaries`], shopItm)
			continue
		}

		if shopItm.PetType != `` {
			shopData[`Pets`] = append(shopData[`Pets`], shopItm)
			continue
		}
	}
	tplData[`mobShop`] = shopData

	tplData[`characterInfo`] = &mobInfo.Character
	tplData[`allZoneNames`] = allZoneNames
	tplData[`allSpecies`] = allSpecies
	tplData[`activityLevels`] = activityLevels
	tplData[`dropChances`] = dropChances
	// -1 marks an absent itemdropchance (the default) so the page can tell it
	// from an authored 0, which means worn items never drop (#585).
	mobDropChance := -1
	if mobInfo.ItemDropChance != nil {
		mobDropChance = *mobInfo.ItemDropChance
	}
	tplData[`mobDropChance`] = mobDropChance
	tplData[`allMobGroups`] = allMobGroups
	tplData[`conditionSpecs`] = conditionSpecs

	if err := tmpl.Execute(w, tplData); err != nil {
		mudlog.Error("HTML Execute", "error", err)
	}

}
