# eu5-iv2-korean

EU5 모드 [The Idea Variation 2](https://steamcommunity.com/sharedfiles/filedetails/?id=3599735023)(워크샵 `3599735023`) 한글 번역 + 번역 관리 툴(`iv2loc`).

기존 한글패치가 크래시나서 새로 만듦.

- 기준 IV2: **2.0.5** (2026-07-14, flogi) / 게임 1.3.11
- 번역 완료. 유니크 문자열 2,766개 → 게임 키 10,513개 한글

## 설치

[Release](https://github.com/gyulsbox/eu5-iv2-korean/releases)에서 zip 받아서
`iv2_korean` 폴더를 여기다 넣고 런처에서 켜기. 로드 순서는 IV2 뒤.

```
%USERPROFILE%\Documents\Paradox Interactive\Europa Universalis V\mod\
```

**경로에 한글 들어가면 모드 안 읽힘.** 게임 언어를 한국어로.

## 빌드

`catalog.json`에 번역이 들어있어서 **API 키 없이 바로 됨.**

```bash
go build -o iv2loc ./cmd/iv2loc

./iv2loc build \
  --src "C:/Program Files (x86)/Steam/steamapps/workshop/content/3450310/3599735023" \
  --out "$USERPROFILE/Documents/Paradox Interactive/Europa Universalis V/mod/iv2_korean" \
  --catalog catalog.json
```

빌드 끝나면 `validate` 자동으로 돌고, 문제 있으면 exit 1.

## IV2 업데이트 오면

```bash
./iv2loc diff      --src <새 IV2> --catalog catalog.json --out <팩> --update
./iv2loc translate --catalog catalog.json --glossary glossary.json   # API 키 필요
./iv2loc build     --src <새 IV2> --out <팩> --catalog catalog.json
```

번역을 키가 아니라 **영어 원문 기준**으로 들고 있어서, 원문 안 바뀐 건 번역 그대로 남음.
테스트해보니 문구 1개 수정 + 2개 추가 + 1개 삭제 = 2,766개 아니라 **3개**만 다시 번역.

## 번역 고치기

- 문장 하나 → `catalog.json`에서 `korean` 고치고 다시 `build`
- 용어 통일 → `glossary.json`에 넣고 `translate --retranslate`

`members`가 그 문장이 게임에서 몇 개 키에 쓰이는지 알려줌.

```json
{ "template": "Level ⟦N1⟧", "korean": "레벨 ⟦N1⟧",
  "rep": "iv2_level_1", "class": "prose", "members": 7 }
```

## 나머지 명령

```bash
./iv2loc extract  --src <IV2>                # 키 몇 개, 번역 대상 몇 개
./iv2loc validate --src <IV2> --out <팩>     # 토큰 깨짐 + 본체 키 침범 검사
./iv2loc polish   --catalog catalog.json     # 조사 기계적 교정
./iv2loc keys     --src <트리> --out k.txt   # 로컬 트리를 키 이름만으로
```

`-h` 붙이면 플래그 나옴. `translate`는 기본 `claude-haiku-4-5`, 전체 돌려도 1달러 안 됨.
`--limit 50`으로 맛보기, `--dry-run`으로 프롬프트만 확인.

## 나중에 까먹을 것들

- **UTF-8 BOM 없으면 EU5가 파일을 그냥 무시함.** 에러도 안 뜸. 출력할 때 항상 붙이고 테스트에서 바이트로 확인함.
- 헤더는 `l_korean:`. `korean:`으로 쓰면 파싱이 안 돼서 키 전부 무효인데 파일은 멀쩡해 보임. (한 번 당함)
- 역설사 `.yml`은 YAML 아님. 값 안의 `#bold`가 YAML 주석이랑 겹쳐서 표준 파서 쓰면 망가짐. 그래서 직접 파싱.
- 출력 파일에 `0_` 접두사 붙는 건 일부러임. EU5는 역알파벳순으로 읽고 먼저 정의된 키가 이기니까, `0_`이면 우리가 꼴찌 → 딴 데서 이미 정의했으면 그쪽이 이김. 우리는 IV2에 없는 한글 레이어 채우는 거라 이게 맞음.
- `⟦T1⟧을(를)` 짝 조사는 게임이 런타임에 값 넣는 자리라 받침을 모름. 숫자(`⟦N1⟧`)는 build 때 값을 알게 돼서 거기서 `4로`로 풀림.
- IV2 원문에 닫는 따옴표 빠진 값 8개 있음. 파서가 줄 끝에서 복구함.
- 영어로 남은 문자열 7개는 의도한 거 (`EV`/`DB` 디버그, IV2 원문이 깨진 것, `Inter Caetera`).

## 개발

```bash
go test ./...
```

`paradox`(파서·토큰) → `inventory`(그룹핑) → `validate`(검사·폴백) → `korean`(조사) → `translate`(API) → `build`(팩 생성) → `diff`(업데이트 비교)

핵심 아이디어 하나: 마크업(`$VAR$`, `[Scope.Func]`, `#bold` 등)이랑 숫자를 `⟦T1⟧`/`⟦N1⟧`로 빼내고 번역함.
그래서 (1) 마크업 깨질 일 없고 (2) 마크업/숫자만 다른 문자열이 하나로 합쳐져서 키 50,052개 → 번역 2,766개.
번역문의 토큰 개수가 원문과 다르면 그 키는 영어로 되돌림 → 번역 틀려도 게임은 안 죽음.
