# Cap-110 mall avatars

Port-only, not native. A newer (cap-110) client ships mall avatar outfits that
v1.150 never had. `scripts/import_cap110_avatars.py` copies a whole set from
that client into the v1.150 extraction, and `SRO_MALL_EXTRA_GOODS=on,cap110`
lists it in the Item Mall. Without the import, or with the setting left at its
default `on`, the game is the v1.150 game exactly.

The first set is **Cursed Soul** (`piratecrew`): dress, hat and back
accessory, male and female, each with a recolour.

## What the import writes

Run it once against the game root of the existing v1.150 extraction, with the
folder holding the newer client's `Data.pk2` and `Media.pk2`:

```powershell
$env:SRO_GAME_ROOT = 'C:\sroserver\client'
py -3 scripts/import_cap110_avatars.py 'C:\path\to\VSRO_Client' piratecrew
```

`--dry-run` lists what would change. The archives are only read. A second run
repairs the same files and rows and never duplicates one.

| Where | What |
| --- | --- |
| `extracted/Data_extracted` | the sets' models (`res/item/avatar`), meshes, materials and textures |
| `extracted/Media_extracted/icon/item/avatar` | the inventory and mall icons |
| `textdata/ItemData_30000.txt` | the item rows, listed in `itemdata.txt` |
| `textdata/textdataname.txt` | English names and tooltips, appended |
| `textdata/refpackageitem.txt`, `refpricepolicyofitem.txt`, `refscrapofpackageitem.txt` | the mall packages, prices and scraps, appended |

The donor's item IDs are kept. The newer client reuses some IDs for other items
than v1.150 does, so the import refuses an ID (item, package or scrap) the
extraction already uses. Only mall rows of the named set are taken; the donor's
event copies are left out. The donor's price rows carry one extra cell, which
the import drops; every mall row takes the extraction's service value and the
silk price of its tier.

Never replace the v1.150 textdata wholesale with the newer client's tables:
the shared rows differ.

## Build and list

```powershell
pnpm assets build
pnpm task build server-game-data
```

Then start GameWorld with `SRO_MALL_EXTRA_GOODS=on,cap110` (Nomad variable
`mall_extra_goods`). The setting is a comma list; `on` is the v1.150 outfits
the mall never sold, `cap110` is this set, and `tab:package` entries still work.
If the packages were not imported, GameWorld stops at boot and the message
names the setting and this script.

## Items and prices

The mall prices follow the v1.150 outfit tiers, not the donor's later ones:
130 silk for a dress, 30 for a hat and 15 for the back accessory. The attachment is listed on the hat tab,
as Halloween's is, because v1.150 has no attachment tab.

| | Male | Female |
| --- | --- | --- |
| Dress | `ITEM_MALL_AVATAR_M_PIRATECREW` (24357) | `ITEM_MALL_AVATAR_W_PIRATECREW` (24358) |
| Hat | `ITEM_MALL_AVATAR_M_PIRATECREW_HAT` (24353) | `ITEM_MALL_AVATAR_W_PIRATECREW_HAT` (24354) |
| Accessory | `ITEM_MALL_AVATAR_M_PIRATECREW_ATTACH` (24363) | `ITEM_MALL_AVATAR_W_PIRATECREW_ATTACH` (24364) |
| Recolour dress | `ITEM_MALL_AVATAR_M_PIRATECREW_2` (35580) | `ITEM_MALL_AVATAR_W_PIRATECREW_2` (35581) |
| Recolour hat | `ITEM_MALL_AVATAR_M_PIRATECREW_HAT_2` (35578) | `ITEM_MALL_AVATAR_W_PIRATECREW_HAT_2` (35579) |

Outside the mall, a GM can create one with `/MAKEITEM <codename> 1`.

## Verification scope

The importer's row handling has unit tests (`scripts/test/python/test_cap110_avatars.py`).
The setting's parsing and its refusal without the import are tested without
game data; the test that lists the imported set in a real catalogue runs only
where the import was applied. Equipping, the mall window and the models were
not played through in a client for this change.
