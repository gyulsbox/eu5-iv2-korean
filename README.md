# iv2loc

Toolchain for building and maintaining a Korean localization pack for the
Europa Universalis V mod [The Idea Variation 2][iv2] (workshop ID
`3599735023`).

[iv2]: https://steamcommunity.com/sharedfiles/filedetails/?id=3599735023

## Why this exists

The existing community Korean pack crashes EU5. The cause is not the
translation, it is the packaging: it ships 12 files into
`localization/korean/`, only one of which is about IV2. The other 11 overwrite
base-game and third-party mod localization, producing 153 key collisions —
42 in `cw_settings_l_korean.yml`, 18 in `languages.yml`, 16 under
`jomini/multiplayer_gui/`. EU5 already ships official Korean; a stale pack
built against version 1.2 stomps it, and the game hard-crashes about 26
seconds in during mod script validation.

So the first principle here is narrow and absolute:

> **Translate only the keys IV2 itself defines. Never touch a base-game key.**

## Status

`extract` and `validate` are implemented. `diff`, `translate` and `build` are
not yet written.

## Usage

```
iv2loc extract  --src <IV2 path> [--json inventory.json]
iv2loc validate --src <IV2 path> [--out <pack>] [--baseline <tree>]...
```

### extract

| flag | meaning |
|---|---|
| `--src` | path to the IV2 mod directory (required) |
| `--lang` | source language to read (default `english`) |
| `--json` | write the full inventory, including every translation group, to a file |
| `--top` | how many of the largest prose groups to list (default 15) |
| `--show-foreign` | list every un-namespaced key shape instead of the top 25 |

### validate

| flag | meaning |
|---|---|
| `--src` | path to the IV2 mod directory (required) |
| `--out` | the generated Korean pack; omit to run source-side checks only |
| `--baseline` | a localization tree we must not redefine; repeatable |
| `--json` | write the full report to a file |
| `--limit` | findings to print per rule (default 20) |

Exits non-zero when there is any blocking finding, so it drops straight into
a build script.

## What validate enforces

Every rule below is covered by a test that feeds it a violation and asserts it
is caught.

| rule | severity | meaning |
|---|---|---|
| `token-mismatch` | error | the translation lost, gained or renamed markup |
| `leaked-placeholder` | error | a `⟦T1⟧` survived into the output; build failed to substitute |
| `empty-translation` | error | a non-empty source became an empty string |
| `do-not-translate` | error | an engine token like a colour name was translated |
| `unknown-key` | error | the pack defines a key IV2 does not |
| `shadows-baseline` | error | the pack defines a key the base game or another mod owns |
| `missing-bom` | error | the game would skip the file silently |
| `bad-header` | error | first line is not `l_korean:` |
| `bad-filename` | error | name does not end in `_l_korean.yml`, or the file is outside `localization/korean/` |
| `replace-dir` | error | a key sits under `replace/`, which should never be necessary |
| `duplicate-key` | warn | defined in more than one output file |
| `untranslated` | warn | prose identical to the English source |
| `malformed-source` | warn | IV2's own defect, recovered |

The central one is `token-mismatch`. Its invariant is that the multiset of
markup tokens in a source string equals the multiset in its translation. The
multiset is order-insensitive on purpose: Korean word order moves clauses, and
a translation that reorders `$A$` and `$B$` is correct while one that drops
`$B$` is not.

Any key that fails validation is listed in `fallback_keys`, and
`validate.Fallback` returns the English source for it. That is the guarantee
worth stating plainly: **a bad translation can make the game read wrong, but it
cannot make the game crash.**

### The collision check needs your files

`validate` cannot prove the pack redefines nothing until it is given the trees
to compare against. With no `--baseline` it says so rather than reporting a
pass:

```
═══ base-game / foreign-mod collisions ═══
  NOT CHECKED - no --baseline given.
```

To prove it, pass the base game and every other mod in the load order:

```
iv2loc validate --src <IV2> --out <pack> \
  --baseline "<EU5>/game" \
  --baseline "<workshop>/<FUM id>" \
  --baseline "<workshop>/<Glorp UI id>" \
  --baseline "<workshop>/<CMM id>"
```

Baselines are scanned in both English and Korean, because a key the base game
defines in either language is a key we must not touch.

## What extract found in IV2 2.0.5

```
total keys defined               50052
unique values (exact)            18616
translation groups                2777

CLASS                      KEYS     GROUPS
empty                      4221          1
reference-only            35249         10
prose                     10582       2766
```

**2,766 strings need translating**, about 133 KB of source prose — an 18x
reduction from the raw key count. Two mechanisms get it there.

**Reference-only values.** 35,249 keys (70%) hold no prose at all. They are
pure `$key$` chains that the engine expands at display time, e.g.

```
WE_PERFORM_iv2_ga_select_ideagroup_adm_2_ACTION_LOG: "$iv2_message_selected_ig$ $iv2_ideagroup_title_adm_2$"
```

Translating the handful of leaf strings these reference translates all of
them. They collapse into 10 groups.

**Placeholder templating.** Before grouping, every markup token becomes `⟦T1⟧`,
`⟦T2⟧` … and every remaining digit run becomes `⟦N1⟧`, `⟦N2⟧` …:

```
"Game Rule $setting_iv2_bonus_3$ gives 10%"  ->  "Game Rule ⟦T1⟧ gives ⟦N1⟧%"
```

Strings differing only in their markup or numbers collapse into one unit. The
placeholders are indexed rather than positional, so a Korean translation is
free to reorder them and reconstruction still puts the right value back.

This is verified lossless: `Templatize` → `Detemplatize` and `Value` → `Quote`
→ `Parse` both round-trip byte for byte across all 50,052 real strings.

## Findings that constrain the later stages

**340 un-namespaced key shapes (6,249 keys).** These carry no `iv2` marker and
are indistinguishable from base-game keys by inspection:

```
STATIC_MODIFIER_NAME_national_idea_modifier_adm_2_1
ADD_IDEAGROUP_SLOT_TOOLTIP_ADM
IV_IDEAGROUP_COST_TT_ADM
```

They are engine-prefixed keys naming IV2-owned objects, so they are almost
certainly ours to translate — but "almost certainly" is what killed the last
pack. `validate --baseline` settles it, and until it is run against the real
trees this stays unproven.

**62 keys must never be translated,** and `validate` refuses them. 26 are
`iv2_*_color`, whose value is the literal string `green` or `yellow` that the
engine looks up as a colour. The other 36 hold their own key name as their
value, e.g. `iv2_idea_alert: "iv2_idea_alert"`. Whether those are engine
lookups or an upstream oversight, copying them verbatim gives Korean players
exactly what English players see, while translating them risks breaking an
alert. Every one was checked by hand; the rule has no false positives on
IV2 2.0.5.

**8 keys are malformed upstream.** IV2 ships eight values with a missing
closing quote, all in `01_IV2_setup_l_english.yml`:

```
iv2_appoint_researcher_adm_act_past: "[SCOPE.sCharacter('recipient').GetName] was appointed as [iv2_researcher_adm|e].
```

The parser recovers them at end of line and flags them rather than dropping the
keys.

**Korean particles are an open problem.** A template like `⟦T1⟧를 선택했습니다`
picks the wrong particle depending on whether the substituted noun ends in a
consonant. EU5's own Korean uses the `을(를)` form. Whatever `translate` emits
has to follow that convention.

## EU5 localization rules this tool must honour

| rule | detail |
|---|---|
| encoding | UTF-8 **with BOM**. Without it the game silently ignores the file. |
| file name | must end in `_l_korean.yml` |
| load order | files are processed in reverse alphabetical order; a leading `0` applies last |
| overwriting | a later key does not override an earlier one; overriding requires `localization/korean/replace/` |
| header | first line is `l_korean:` |
| key form | `key:0 "text"`, where the `:0` version number is optional |
| layout | `<mod>/{in_game,main_menu,loading_screen}/localization/korean/*.yml` |
| metadata | `.metadata/metadata.json` plus `.metadata/thumbnail.png` |
| install path | `%USERPROFILE%/Documents/Paradox Interactive/Europa Universalis V/mod/<name>/` |
| path charset | ASCII only — non-ASCII mod paths fail to load |

Note that IV2 puts all its localization under `main_menu/`, and that we are
translating keys IV2 defines rather than overriding anything, so `replace/`
should never be needed. If it starts looking necessary, that is a signal we
have wandered into base-game keys and the design needs revisiting.

## Development

```
go test ./...
go run ./cmd/iv2loc extract --src <IV2 path>
```

`internal/paradox` parses the format and scans markup tokens.
`internal/inventory` walks a mod, classifies values and groups them into
translation units. Fixtures under `internal/inventory/testdata/mod` are trimmed
from real IV2 content and include the BOM-less file, the duplicate key and the
malformed entry that the real mod contains.
