# Merchant chests: roster

Generated 2026-10-01 from the merchant templates, their shop lists and the
item values, with the same formula `internal/merchantchests` applies at run
time. The live data is `_datafiles/world/dogmud/merchant_chests.yaml`.

## Formula

With `d = log2(1 + average stock value)`, where the average is the plain mean
of `value:` over the distinct items in the merchant's `shop:` list:

| Quantity | Formula | Range over the roster |
|---|---|---|
| Perception base | `perception_base + perception_per_doubling x d` = `100 + 4d` | 104 to 129 |
| Lock pins | `lock_base + lock_per_doubling x d` = `4 + 2d`, clamped 6 to 20 | 6 to 18 |
| Chest gold per restock | `average x 0.5 x (1 +/- 0.5)`, at least 1 | |
| Chest goods per restock | 1 to 3 random items from the shop list | |

Perception is the template `stats.perception.base`. A merchant's `statpool`
still adds its random share of points on top at spawn, so a spawned merchant
reads a few points higher than the base in this table.

A sleeping merchant's Perception counts at `sleeping_perception_mult` (half)
in every theft and detection contest.

Veyra Coil-Tongue (9584) is a crafter with no shop list, so she keeps no
chest and her Perception is unchanged.

## Roster

| Mob | Merchant | Zone | Room | Type | Items | Avg value | Perception (was, now) | Chest | Pins | Gold per restock |
|---|---|---|---|---|---|---|---|---|---|---|
| 109 | Enchanter Vael | Thornwall City | 483 Enchanter's Circle | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 9588 | Enchanter Rane | Stillwater | 6443 The Chrysalis Workshop | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 9593 | Cade the Enchanter | Greenford | 6448 The Sigil Room | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 9597 | Merrow the Enchanter | The Confluence | 6452 The Warded Alcove | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 9602 | Sefa Crane | Hartcharn | 6457 The Rune-Carver's Hut | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 9606 | Enchanter Skell | Pothole Coulee | 6461 The Warder's Dugout | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 9610 | Ansel Rune | New Plymouth Crafting | 6465 The Enchanter's Atelier | enchanting | 8 | 144.4 | 115, 129 | coffer | 18 | 36 to 108 |
| 337 | Smith Brindle | Stillwater | 4106 Brindle's Smithy | blacksmithing | 15 | 107.5 | 105, 127 | strongbox | 18 | 27 to 81 |
| 98 | Apothecary Voss | Thornwall City | 471 Apothecary Lane | alchemy | 13 | 63.0 | 115, 124 | cabinet | 16 | 16 to 47 |
| 9592 | Hebb the Herbalist | Greenford | 6447 The Herbalist's Stall | alchemy | 13 | 63.0 | 115, 124 | cabinet | 16 | 16 to 47 |
| 9601 | Alard Fen | Hartcharn | 6456 The Apothecary's Hut | alchemy | 13 | 63.0 | 115, 124 | cabinet | 16 | 16 to 47 |
| 108 | Jeweler Tess | Thornwall City | 482 Jeweler's Workshop | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 340 | Pearl-carver Kess | Stillwater | 4126 Pearl-Carver's Garret | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 9594 | Tibb the Jeweler | Greenford | 6449 The Jeweler's Bench | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 9598 | Bevan the Jeweler | The Confluence | 6453 The Gemcutter's Nook | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 9603 | Til Marsh | Hartcharn | 6458 The Gem Hut | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 9607 | Jeweler Vurl | Pothole Coulee | 6462 The Gem Stall | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 9611 | Perla Gilt | New Plymouth Crafting | 6466 The Jeweler's Atelier | jewelcrafting | 9 | 60.6 | 110, 124 | casket | 16 | 15 to 45 |
| 338 | Apothecary Ilsa | Stillwater | 4125 Healer's Alcove | alchemy | 15 | 56.1 | 112, 123 | cabinet | 16 | 14 to 42 |
| 9345 | Falk the Auctioneer | New Plymouth Merchant | 5808 Falk's Auction House | general | 3 | 26.0 | 112, 119 | vault | 14 | 7 to 20 |
| 103 | Food Vendor | Thornwall City | 464 Market Square, West | cooking | 5 | 23.4 | 100 (species), 118 | cashbox | 13 | 6 to 18 |
| 248 | Tavern Cook Brynn | Thornwall City | 481 Tavern Kitchen | cooking | 10 | 19.4 | 103, 117 | cashbox | 13 | 5 to 15 |
| 9604 | Gerta Crook | Hartcharn | 6459 The Cookhouse | cooking | 10 | 19.4 | 103, 117 | cashbox | 13 | 5 to 15 |
| 9608 | Cook Hesk | Pothole Coulee | 6463 The Cook-Fire | cooking | 10 | 19.4 | 103, 117 | cashbox | 13 | 5 to 15 |
| 273 | Whisper | Thornwall City | 507 The Listening Post | general | 3 | 18.3 | 115, 117 | chest | 13 | 5 to 14 |
| 97 | Blacksmith Kerra | Thornwall City | 470 Craftsmen's Quarter, East | blacksmithing | 7 | 16.1 | 105, 116 | strongbox | 12 | 4 to 12 |
| 9590 | Aldo the Smith | Greenford | 6445 The Guild Forge | blacksmithing | 7 | 16.1 | 105, 116 | strongbox | 12 | 4 to 12 |
| 9596 | Roan the Smith | The Confluence | 6451 The Riverside Forge | blacksmithing | 7 | 16.1 | 105, 116 | strongbox | 12 | 4 to 12 |
| 113 | Weaver Maren | Thornwall City | 480 Tailor's Workshop | tailoring | 4 | 14.5 | 108, 116 | trunk | 12 | 4 to 11 |
| 9591 | Wenna the Weaver | Greenford | 6446 The Weaver's Loft | tailoring | 4 | 14.5 | 108, 116 | trunk | 12 | 4 to 11 |
| 9600 | Bryd Harrow | Hartcharn | 6455 The Weaving Shed | tailoring | 4 | 14.5 | 108, 116 | trunk | 12 | 4 to 11 |
| 9605 | Stitcher Bex | Pothole Coulee | 6460 The Patch-Tent | tailoring | 4 | 14.5 | 108, 116 | trunk | 12 | 4 to 11 |
| 9307 | Chandler Voss | New Plymouth Docks | 5508 The Chandlery | general | 7 | 13.6 | 108, 115 | chest | 12 | 3 to 10 |
| 336 | Fishmonger Tov Brann | Stillwater | 4102 Lakefront Square | cooking | 4 | 13.0 | 106, 115 | cashbox | 12 | 3 to 10 |
| 9347 | Dame Ostry | New Plymouth Merchant | 5810 Dame Ostry's Armoury | blacksmithing | 5 | 13.0 | 100 (species), 115 | strongbox | 12 | 3 to 10 |
| 9348 | Brun the Armorer | New Plymouth Merchant | 5812 Brun's Plate-Works | blacksmithing | 5 | 12.8 | 100 (species), 115 | strongbox | 12 | 3 to 10 |
| 250 | Peddler Malk | Marches Spur Road | 4003 Peddler's Camp | fence only | 3 | 12.0 | 108, 115 | lockbox | 11 | 3 to 9 |
| 9172 | Sly Tam | North Road North | 5378 The Lake & Ladle, Common Room | fence only | 3 | 12.0 | 108, 115 | lockbox | 11 | 3 to 9 |
| 9215 | A River-Road Smuggler | New Plymouth Outskirts | 5479 The Ford | fence only | 3 | 12.0 | 110, 115 | lockbox | 11 | 3 to 9 |
| 9323 | Ysolde | New Plymouth Common | 5620 Ysolde's Room | fence only | 3 | 12.0 | 108, 115 | lockbox | 11 | 3 to 9 |
| 339 | Weaver Edda | Stillwater | 4143 Tailor's Cottage | tailoring | 3 | 11.0 | 107, 114 | trunk | 11 | 3 to 8 |
| 341 | Storekeeper Wulf | Stillwater | 4105 Tinder & Tackle | general | 9 | 9.9 | 108, 114 | chest | 11 | 2 to 7 |
| 104 | Fence Dealer Siv | Thornwall City | 475 Back Alley, East | general, fence | 6 | 9.7 | 110, 114 | lockbox | 11 | 2 to 7 |
| 9104 | Trader Onna | Pothole Coulee | 5207 Coulee Provisions | general | 14 | 8.8 | 107, 113 | chest | 11 | 2 to 7 |
| 9306 | Old Sable | New Plymouth Docks | 5518 Old Sable's Pawnshop | general | 5 | 8.6 | 112, 113 | safe | 11 | 2 to 6 |
| 9337 | Corwin the Tanner | New Plymouth Crafting | 5717 The Tannery | tailoring | 1 | 8.0 | 100 (species), 113 | trunk | 10 | 2 to 6 |
| 9475 | The Weaver | The Confluence | 6235 The Weaver's | tailoring | 2 | 7.0 | 103, 112 | trunk | 10 | 2 to 5 |
| 9428 | Varro the Importer | The Confluence | 6128 The Spice Quay | cooking, fence | 4 | 6.8 | 108, 112 | lockbox | 10 | 2 to 5 |
| 9510 | Daven the Innkeeper | Greenford | 6291 The Cartographer's Rest | cooking | 2 | 6.5 | 105, 112 | cashbox | 10 | 2 to 5 |
| 9191 | Tamsin Reed | Greywater Flats | 5425 Ford Approach | general | 4 | 6.2 | 108, 111 | chest | 10 | 2 to 5 |
| 9444 | Drunn the Bookseller | The Confluence | 6226 The Bookseller's | general | 1 | 6.0 | 105, 111 | chest | 10 | 2 to 5 |
| 9473 | The Cooper | The Confluence | 6234 The Cooperage | general | 1 | 6.0 | 103, 111 | chest | 10 | 2 to 5 |
| 9474 | The Potter | The Confluence | 6236 The Potter's | alchemy | 1 | 6.0 | 104, 111 | cabinet | 10 | 2 to 5 |
| 9509 | Aldith the Bookseller | Greenford | 6290 The Bookshop | general | 1 | 6.0 | 106, 111 | chest | 10 | 2 to 5 |
| 9512 | A Produce-Seller | Greenford | 6289 The Market Cross | cooking | 1 | 6.0 | 105, 111 | cashbox | 10 | 2 to 5 |
| 9182 | Severin Pell | Hartcharn | 5409 The Ferry and Coach Agent | general | 6 | 5.7 | 112, 111 | chest | 9 | 1 to 4 |
| 9311 | Corra | New Plymouth Docks | 5516 The Cookshop | cooking | 3 | 5.7 | 100 (species), 111 | cashbox | 9 | 1 to 4 |
| 9505 | Fenn the Fishmonger | Greenford | 6282 Riverside Row | cooking | 3 | 5.7 | 106, 111 | cashbox | 9 | 1 to 4 |
| 9196 | Brannick Oats | Kingsbarrow Vale | 5449 The Sheaf and Sickle | general | 5 | 5.6 | 108, 111 | chest | 9 | 1 to 4 |
| 9477 | The Craft-Supply Seller | The Confluence | 6240 The Craft-Supply Stall | general | 2 | 5.5 | 106, 111 | chest | 9 | 1 to 4 |
| 9525 | Ness the Tea-Keeper | Greenford | 6312 The Tea House | cooking | 2 | 5.5 | 105, 111 | cashbox | 9 | 1 to 4 |
| 9531 | Wren the Ostler | Greenford | 6319 The Coaching Stable | cooking | 2 | 5.5 | 105, 111 | cashbox | 9 | 1 to 4 |
| 9301 | Bressa Toll | New Plymouth Docks | 5512 The Salt Cellar Taproom | cooking | 5 | 5.4 | 108, 111 | cashbox | 9 | 1 to 4 |
| 9322 | Renn Bowl | New Plymouth Common | 5607 The Brimming Bowl Taproom | cooking | 5 | 5.4 | 108, 111 | cashbox | 9 | 1 to 4 |
| 9171 | Goodwife Pemberton | North Road North | 5378 The Lake & Ladle, Common Room | general | 6 | 5.3 | 108, 111 | chest | 9 | 1 to 4 |
| 9502 | Maret the Miller | Greenford | 6280 The Watermill | cooking | 3 | 5.3 | 106, 111 | cashbox | 9 | 1 to 4 |
| 9174 | Old Mabbot | North Road North | 5373 The Rolling Pasture Road | general | 7 | 5.3 | 108, 111 | chest | 9 | 1 to 4 |
| 9321 | Marda | New Plymouth Common | 5605 Cookshop Row | cooking | 4 | 5.2 | 100 (species), 111 | cashbox | 9 | 1 to 4 |
| 278 | Haral | North Road | 4045 Common Room | cooking | 1 | 5.0 | 106, 110 | cashbox | 9 | 1 to 4 |
| 9209 | A Market Hawker | New Plymouth Outskirts | 5467 The Outer Market | general, fence | 7 | 5.0 | 108, 110 | lockbox | 9 | 1 to 4 |
| 9213 | Mother Coyle | New Plymouth Outskirts | 5475 The Broken Gate | general, fence | 3 | 5.0 | 108, 110 | lockbox | 9 | 1 to 4 |
| 9421 | Pella the Fish-Trader | The Confluence | 6110 The Fish Quay | cooking | 5 | 5.0 | 108, 110 | cashbox | 9 | 1 to 4 |
| 9451 | The Offering-Seller | The Confluence | 6155 Processional Avenue, Votive Stalls | general | 3 | 5.0 | 104, 110 | coffer | 9 | 1 to 4 |
| 9492 | Mistress Odell the Victualler | East Road to Greenford | 6268 The Wheatside Hamlet | cooking | 3 | 5.0 | 106, 110 | cashbox | 9 | 1 to 4 |
| 9181 | Maret Cull | Hartcharn | 5406 The Coachman's Rest, Common Room | general | 7 | 4.9 | 108, 110 | chest | 9 | 1 to 4 |
| 102 | Market Merchant | Thornwall City | 465 Market Square, Center | general | 10 | 4.8 | 105, 110 | chest | 9 | 1 to 4 |
| 9116 | Smith Rusk | Pothole Coulee | 5245 The Coulee Smithy | blacksmithing | 4 | 4.8 | 106, 110 | strongbox | 9 | 1 to 4 |
| 9185 | Wick Orrel | Hartcharn | 5415 The Tap and Trough | general, fence | 4 | 4.8 | 111, 110 | lockbox | 9 | 1 to 4 |
| 9391 | Almoner Sten | New Plymouth Temple | 5903 The Temple Gate Plaza | general | 4 | 4.8 | 100 (species), 110 | coffer | 9 | 1 to 4 |
| 9350 | Madam Sephe | New Plymouth Merchant | 5814 The Gilt Threshold | general | 3 | 4.7 | 106, 110 | coffer | 9 | 1 to 4 |
| 9128 | Herbalist Birna | Pothole Coulee | 5265 The Sheltered Pool | alchemy | 4 | 4.5 | 112, 110 | cabinet | 9 | 1 to 3 |
| 9309 | Fishmonger | New Plymouth Docks | 5504 The Fish Market | cooking | 4 | 4.5 | 106, 110 | cashbox | 9 | 1 to 3 |
| 9412 | Birrel the Netmender | River Road | 6098 Netmender's Row | cooking | 4 | 4.5 | 106, 110 | cashbox | 9 | 1 to 3 |
| 85 | Merchant Brecca | Watchers Crossing | 424 Trading Post | general | 10 | 4.4 | 110, 110 | chest | 9 | 1 to 3 |
| 9334 | Vesna | New Plymouth Crafting | 5711 The Alchemist's Lab | alchemy | 3 | 4.3 | 108, 110 | cabinet | 9 | 1 to 3 |
| 9333 | Master Halvard | New Plymouth Crafting | 5709 The Forge Yard | blacksmithing | 4 | 4.2 | 100 (species), 110 | strongbox | 9 | 1 to 3 |
| 9373 | Modiste Aurel | New Plymouth Noble | 6010 Modiste Aurel's Atelier | tailoring | 4 | 4.2 | 108, 110 | trunk | 9 | 1 to 3 |
| 9203 | Sela Tapp | Kilnreach Works | 5465 The Furnace and Flagon | general | 6 | 4.2 | 110, 109 | chest | 9 | 1 to 3 |
| 9184 | Ondine | Hartcharn | 5411 The General Store | general | 9 | 4.1 | 110, 109 | chest | 9 | 1 to 3 |
| 9429 | Lenne the Provisioner | The Confluence | 6127 The River Market | general | 4 | 4.0 | 107, 109 | chest | 9 | 1 to 3 |
| 9511 | Prue the Store-Keeper | Greenford | 6293 The General Store | general | 3 | 4.0 | 107, 109 | chest | 9 | 1 to 3 |
| 9303 | Dunmar Wells | New Plymouth Docks | 5505 Warehouse Row | general | 6 | 3.8 | 114, 109 | chest | 9 | 1 to 3 |
| 9197 | Dawkin the Miller | Kingsbarrow Vale | 5448 The Watermill | cooking | 4 | 3.8 | 109, 109 | cashbox | 8 | 1 to 3 |
| 9305 | Marn the Draper | New Plymouth Docks | 5523 Marn's Fabric-Remnants Shop | tailoring | 8 | 3.6 | 108, 109 | trunk | 8 | 1 to 3 |
| 9486 | The Innkeeper | The Confluence | 6252 The Travelers' Inn | cooking | 3 | 3.3 | 104, 108 | cashbox | 8 | 1 to 3 |
| 9423 | Ferrick the Chandler | The Confluence | 6114 The Chandlery | general | 4 | 3.2 | 106, 108 | chest | 8 | 1 to 2 |
| 9332 | Orin the Bookseller | New Plymouth Crafting | 5706 Orin's Bookstall | general | 1 | 3.0 | 114, 108 | chest | 8 | 1 to 2 |
| 9335 | Edda Glass | New Plymouth Crafting | 5713 The Glass Kiln | general | 1 | 3.0 | 106, 108 | chest | 8 | 1 to 2 |
| 9472 | The Baker | The Confluence | 6238 The Baker's | general | 2 | 3.0 | 105, 108 | chest | 8 | 1 to 2 |
| 9336 | Nessa the Tailor | New Plymouth Crafting | 5715 The Tailor's Shop | tailoring | 3 | 2.7 | 100 (species), 107 | trunk | 8 | 1 to 2 |
| 348 | Miller Bram | Stillwater | 4135 The Watermill | cooking | 2 | 2.5 | 105, 107 | cashbox | 8 | 1 to 2 |
| 9452 | The Hospitaller | The Confluence | 6160 The Pilgrim Hall | general | 2 | 2.5 | 100 (species), 107 | coffer | 8 | 1 to 2 |
| 9424 | Sybba the Tavern-keeper | The Confluence | 6116 The Quayside Tavern | cooking | 3 | 2.3 | 105, 107 | cashbox | 7 | 1 to 2 |
| 9183 | Drostan | Hartcharn | 5410 The Smithy | blacksmithing | 5 | 2.2 | 104, 107 | strongbox | 7 | 1 to 2 |
| 88 | Traveling Merchant | Watchers Crossing | 423 The Crossing Inn | general | 6 | 2.0 | 105, 106 | chest | 7 | 1 to 2 |
| 9437 | Corliss the Shopkeeper | The Confluence | 6150 The Civic Stores | general | 5 | 2.0 | 107, 106 | chest | 7 | 1 to 2 |
| 9390 | Mardle the Sundries-Seller | New Plymouth Common | 5604 The Common Market | general | 6 | 1.8 | 108, 106 | chest | 7 | 1 to 1 |
| 9326 | Flower Seller | New Plymouth Common | 5602 Carter's Rise | general | 1 | 1.0 | 106, 104 | chest | 6 | 1 to 1 |
| 9326 | Flower Seller | New Plymouth Common | 5603 The Flower Market | general | 1 | 1.0 | 106, 104 | chest | 6 | 1 to 1 |
