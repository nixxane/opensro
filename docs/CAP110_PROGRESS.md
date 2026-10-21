

**Root cause and review (2026-10-21):** the server already sends the monster's
current HP when it is selected (0xB45A, `EncodeMonsterObjectSelectResult`), and
the client already stores it (`hp-seed`). The bar was missing only because
grade 8 was not in the client's grade table, which left the maximum HP unknown.
So the only client change is the grade-8 entry (unique title, icon, x1 HP). No
"assume full HP" fallback and no Experimental row: the earlier fallback was
removed because the native client reads HP from the server (it has no such
default), and the entry is unreachable on a native build (native monster
references are grades 0 and 3; nests add 1, 4, 5 and 6; none is 8).
