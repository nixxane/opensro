"""
===========================================================================

import_cap110_avatars.py - bring mall avatar sets from a newer client into the
v1.150 extraction

The port's items, names and models come from the v1.150 extraction. A newer
(cap-110) client carries mall avatar sets that v1.150 never shipped. This tool
imports whole sets, for example `piratecrew`, from that client:

	<donor>/Data.pk2   models, materials, textures -> extracted/Data_extracted/
	<donor>/Media.pk2  inventory icons             -> extracted/Media_extracted/
	<donor>/Media.pk2  item rows                   -> textdata/ItemData_30000.txt
	<donor>/Media.pk2  English names, tooltips     -> textdata/textdataname.txt
	<donor>/Media.pk2  Item Mall packages, prices  -> textdata/refpackageitem.txt,
	                   and scraps                     refpricepolicyofitem.txt,
	                                                  refscrapofpackageitem.txt

The packages take the v1.150 outfit prices (dress 130, hat 30, attachment 15
silk), not the donor's. They are only listed in the
mall when the server runs with SRO_MALL_EXTRA_GOODS=on,cap110.

The donor's item IDs are kept. They must be unused in the extraction (v1.150
reuses some low IDs for different items), so a collision stops the import.
Only mall avatar rows of the named set are taken; event copies are left out.
Nothing is written into either archive. Running it again repairs the same
files and rows, and never duplicates one.

	py -3 scripts/import_cap110_avatars.py <donor client folder> piratecrew [more sets]
	py -3 scripts/import_cap110_avatars.py <donor client folder> piratecrew --dry-run

The game root is SRO_GAME_ROOT or the parent of the checkout (sro_paths.py).

===========================================================================
"""
import argparse
import mmap
import re
import sys
from pathlib import Path

import sro_pk2
from prepare_client_resources import safe_target, write_atomically
from sro_paths import GAME_ROOT

TEXTDATA = "server_dep/silkroad/textdata"
ITEM_FILE = "ItemData_30000.txt"
NAME_FILE = "textdataname.txt"
# Column layout shared by the donor and the extraction (160 cells).
ID, CODE, NAME_KEY, DESC_KEY = 1, 2, 5, 6
# Donor name rows: active flag, symbol, Korean, ..., English at 8.
DONOR_ENGLISH = 8
# Extraction name rows: active flag, symbol, Korean, 2 blanks, 0, blank, 0, English.
LOCAL_ENGLISH = 8
# Mall tables: file -> cell holding the package codename, and the row width
# once the donor row is adapted. The v1.150 mall rows all carry service 11.
PACKAGE_TABLES = {"refpackageitem.txt": 3, "refpricepolicyofitem.txt": 2, "refscrapofpackageitem.txt": 2}
PACKAGE_WIDTHS = {"refpackageitem.txt": 17, "refpricepolicyofitem.txt": 13, "refscrapofpackageitem.txt": 29}
LOCAL_SERVICE = "11"
# Silk prices of the v1.150 outfit tiers (the European wedding set's): the
# donor's later prices are not used.
SILK_CURRENCY = "2"
SILK_ATTACH, SILK_HAT, SILK_DRESS = "15", "30", "130"
PACKAGE_ID = 2       # refpackageitem: unique package ID
SCRAP_ID = -1        # refscrapofpackageitem: unique scrap ID, the last cell


#============================================================================
# read_text / encode_text
#============================================================================
def read_text(data):
	if data[:2] == b"\xff\xfe":
		return data.decode("utf-16")
	return data.decode("cp949")


def records(text):
	"""Rows end in CRLF only; a bare LF belongs to a cell (splitlines would cut it)."""
	rows = text.split("\r\n")
	return rows[:-1] if rows and rows[-1] == "" else rows


def records_of(path):
	"""The cells of every row of one extracted text file."""
	return [line.split("\t") for line in records(read_text(path.read_bytes())) if line]


def encode_text(lines):
	return b"\xff\xfe" + "".join(line + "\r\n" for line in lines).encode("utf-16-le")


#============================================================================
# set_pattern
#============================================================================
def set_pattern(name):
	"""Mall rows and files of one set: dress, hat, attach and numbered recolours."""
	name = re.escape(name.lower())
	return re.compile(rf"avatar_[mw]_{name}(?:_hat|_attach)?(?:_\d+)*$", re.IGNORECASE)


#============================================================================
# donor_rows
#============================================================================
def donor_rows(media, entries, sets):
	"""Mall item rows of the sets, and the donor's symbol -> name row."""
	patterns = [set_pattern(name) for name in sets]
	items, names = [], {}
	for entry in entries:
		folder, _, base = entry.path.lower().rpartition("/")
		if folder != TEXTDATA:
			continue
		if re.fullmatch(r"itemdata_\d+\.txt", base):
			for line in records(read_text(sro_pk2.payload(media, entry))):
				cells = line.split("\t")
				if len(cells) == 160 and cells[CODE].upper().startswith("ITEM_MALL_AVATAR_") \
					and any(p.search(cells[CODE][len("ITEM_MALL_"):]) for p in patterns):
					items.append(cells)
		elif base == "textdata_object.txt":
			for line in records(read_text(sro_pk2.payload(media, entry))):
				cells = line.split("\t")
				if len(cells) > DONOR_ENGLISH and cells[1].startswith("SN_ITEM_MALL_AVATAR_"):
					names[cells[1]] = cells
	return items, names


#============================================================================
# silk_price
#============================================================================
def silk_price(code):
	"""The v1.150 tier of a package: attachment 15, hat 30, dress 130 silk."""
	stem = re.sub(r"(?:_\d+)+$", "", code)
	if stem.endswith("_ATTACH"):
		return SILK_ATTACH
	return SILK_HAT if stem.endswith("_HAT") else SILK_DRESS


#============================================================================
# package_rows
#============================================================================
def package_rows(media, entries, sets):
	"""
	The sets' Item Mall package, price and scrap rows, in the extraction's
	layout. The donor's price rows carry one extra cell after the currency;
	every row keeps the extraction's service (11) and flag (0) values.
	"""
	patterns = [set_pattern(name) for name in sets]
	rows = {name: [] for name in PACKAGE_TABLES}
	for entry in entries:
		folder, _, base = entry.path.lower().rpartition("/")
		if folder != TEXTDATA or base not in PACKAGE_TABLES:
			continue
		for line in records(read_text(sro_pk2.payload(media, entry))):
			cells = line.split("\t")
			code = cells[PACKAGE_TABLES[base]] if len(cells) > PACKAGE_TABLES[base] else ""
			if not code.upper().startswith("PACKAGE_ITEM_MALL_AVATAR_") \
				or not any(p.search(code[len("PACKAGE_ITEM_MALL_"):]) for p in patterns):
				continue
			cells[1] = LOCAL_SERVICE
			if base == "refpackageitem.txt":
				cells[4] = "0"
			elif base == "refpricepolicyofitem.txt":
				del cells[4]
				if cells[3] == SILK_CURRENCY:
					cells[4] = silk_price(code)
			rows[base].append(cells)
	for name, width in PACKAGE_WIDTHS.items():
		if any(len(cells) != width for cells in rows[name]):
			raise SystemExit(f"{name}: a donor row does not fit the v1.150 layout ({width} cells).")
	return rows


#============================================================================
# main
#============================================================================
def main():
	parser = argparse.ArgumentParser(description="Import mall avatar sets from a newer client.")
	parser.add_argument("donor", type=Path, help="Folder holding the newer client's Data.pk2 and Media.pk2.")
	parser.add_argument("sets", nargs="+", help="Set names such as piratecrew.")
	parser.add_argument("--dry-run", action="store_true", help="List what would change and write nothing.")
	args = parser.parse_args()
	extracted = GAME_ROOT / "extracted"
	text_root = extracted / "Media_extracted" / Path(TEXTDATA)
	if not text_root.is_dir():
		raise SystemExit(f"No v1.150 extraction at {extracted}; run prepare_client_resources.py first.")

	with (args.donor / "Media.pk2").open("rb") as media_handle, \
		mmap.mmap(media_handle.fileno(), 0, access=mmap.ACCESS_READ) as media, \
		(args.donor / "Data.pk2").open("rb") as data_handle, \
		mmap.mmap(data_handle.fileno(), 0, access=mmap.ACCESS_READ) as data:
		media_entries = sro_pk2.read_directory(media)
		data_entries = sro_pk2.read_directory(data)
		items, names = donor_rows(media, media_entries, args.sets)
		if not items:
			raise SystemExit("The donor client has no mall avatar rows for those sets.")
		packages = package_rows(media, media_entries, args.sets)

		# Package and scrap IDs must be free, or already this tool's own rows.
		ours = {cells[PACKAGE_TABLES["refpackageitem.txt"]] for cells in packages["refpackageitem.txt"]}
		for name, id_cell in (("refpackageitem.txt", PACKAGE_ID), ("refscrapofpackageitem.txt", SCRAP_ID)):
			taken = {cells[id_cell] for cells in records_of(text_root / name)
				if cells[PACKAGE_TABLES[name]] not in ours}
			for cells in packages[name]:
				if cells[id_cell] in taken:
					raise SystemExit(f"{name}: ID {cells[id_cell]} is already used in v1.150; refusing {cells[PACKAGE_TABLES[name]]}.")

		# IDs and codes must be free, or already this tool's own rows.
		own = {}
		local = {}
		for path in text_root.glob("[Ii]tem[Dd]ata_*.txt"):
			for line in records(read_text(path.read_bytes())):
				cells = line.split("\t")
				if len(cells) > CODE:
					(own if path.name == ITEM_FILE else local)[cells[ID]] = cells[CODE]
		for cells in items:
			if cells[ID] in local:
				raise SystemExit(f"Item ID {cells[ID]} is already {local[cells[ID]]} in v1.150; refusing {cells[CODE]}.")
			if own.get(cells[ID], cells[CODE]) != cells[CODE]:
				raise SystemExit(f"Item ID {cells[ID]} was imported as {own[cells[ID]]}, not {cells[CODE]}.")

		# Files of the sets: models and textures in Data, inventory icons in Media.
		patterns = [set_pattern(name) for name in args.sets]
		wanted = []
		for entry, root in [(e, "Data_extracted") for e in data_entries] + [(e, "Media_extracted") for e in media_entries]:
			plain = entry.path.rsplit("/", 1)[-1].rsplit(".", 1)[0]
			plain = plain[len("mall_"):] if plain.lower().startswith("mall_avatar_") else plain
			low = entry.path.lower()
			in_scope = low.startswith(("prim/", "res/item/avatar/")) if root == "Data_extracted" \
				else low.startswith("icon/item/avatar/")
			if in_scope and any(p.search(plain) for p in patterns):
				wanted.append((entry, root))

		# Names and tooltips for the rows, in the extraction's own column layout.
		name_lines = []
		for cells in items:
			for key in (cells[NAME_KEY], cells[DESC_KEY]):
				donor = names.get(key)
				if donor is None:
					raise SystemExit(f"The donor has no text for {key}.")
				row = ["1", key, donor[2], "", "", "0", "", "0", donor[DONOR_ENGLISH]] + [""] * 2
				name_lines.append((key, "\t".join(row)))

		print(f"{len(items)} items, {len(wanted)} files, {len(set(k for k, _ in name_lines))} names, "
			f"{len(packages['refpackageitem.txt'])} mall packages")
		for cells in items:
			print(f"  {cells[ID]:>6}  {cells[CODE]}")
		if args.dry_run:
			return

		for entry, root in wanted:
			target = safe_target(extracted / root, entry.path)
			write_atomically(target, bytes(sro_pk2.payload(data if root == "Data_extracted" else media, entry)))

	# Item rows: this tool's own file, kept sorted by ID, listed in the index.
	merged = {cells[ID]: "\t".join(cells) for cells in items}
	item_path = text_root / ITEM_FILE
	if item_path.exists():
		for line in records(read_text(item_path.read_bytes())):
			merged.setdefault(line.split("\t")[ID], line)
	write_atomically(item_path, encode_text([merged[key] for key in sorted(merged, key=int)]))
	index = text_root / "itemdata.txt"
	listed = records(read_text(index.read_bytes()))
	if ITEM_FILE.lower() not in (line.strip().lower() for line in listed):
		write_atomically(index, encode_text(listed + [ITEM_FILE]))

	# Names: replace this tool's rows, keep everything else.
	name_path = text_root / NAME_FILE
	ours = {key for key, _ in name_lines}
	kept = [line for line in records(read_text(name_path.read_bytes())) if line.split("\t")[1:2] not in [[k] for k in ours]]
	write_atomically(name_path, encode_text(kept + list(dict(name_lines).values())))
	# Mall packages: replace this tool's rows in each table, keep everything else.
	for name, code_cell in PACKAGE_TABLES.items():
		path = text_root / name
		codes = {cells[code_cell] for cells in packages[name]}
		kept = [line for line in records(read_text(path.read_bytes()))
			if line.split("\t")[code_cell:code_cell + 1] not in [[code] for code in codes]]
		write_atomically(path, encode_text(kept + ["\t".join(cells) for cells in packages[name]]))
	print("Imported. Rebuild with: pnpm assets build, then pnpm task build server-game-data.")
	print("List the mall goods with SRO_MALL_EXTRA_GOODS=on,cap110 (default off).")


if __name__ == "__main__":
	sys.exit(main())
