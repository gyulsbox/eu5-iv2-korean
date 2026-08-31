# iv2loc

Europa Universalis V 모드 [The Idea Variation 2][iv2](워크샵 ID `3599735023`)의
한국어 번역 모드예요. 번역을 만들고 관리하는 도구도 같이 들어 있습니다.

[iv2]: https://steamcommunity.com/sharedfiles/filedetails/?id=3599735023

별도로 존재하는 한글패치가 있는데 현재 게임 버전에서는 크래시가 나서, 새로 만들었어요.

**번역은 완료된 상태입니다.** 유니크 문자열 2,766개를 번역했고, 실제 게임 키로는
10,513개가 한국어로 나옵니다.

| | |
|---|---|
| 기준 IV2 버전 | **2.0.5** (2026-07-14, flogi) |
| 게임 버전 | 1.3.11 (`supported_game_version` `1.3.*`) |
| 필요 모드 | The Idea Variation 2, Community Mod Framework |

IV2가 이 버전보다 올라가면 [IV2가 업데이트되면](#iv2가-업데이트되면)을 봐주세요.

---

## 설치 (플레이만 하실 경우)

1. `iv2_korean` 폴더를 아래 경로에 넣어주세요.

   ```
   %USERPROFILE%\Documents\Paradox Interactive\Europa Universalis V\mod\iv2_korean\
   ```

   > 폴더 경로에 한글이 들어가면 게임이 모드를 못 읽어요. ASCII 경로로 넣어주세요.

2. 런처에서 활성화하고, 로드 순서를 IV2 뒤에 둡니다.
3. 게임 언어를 한국어로 바꾸면 끝이에요.

빌드된 팩은 생성물이라 저장소에 넣지 않았어요. 아래 방법으로 직접 만들거나
Release에 올라온 zip을 받으시면 됩니다.

---

## 실행 방법

### 준비물

- Go 1.24 이상
- IV2 모드 경로 — 보통 이쯤에 있어요
  `C:\Program Files (x86)\Steam\steamapps\workshop\content\3450310\3599735023`
- Anthropic API 키 — 번역을 새로 돌릴 때만 필요합니다 (`ANTHROPIC_API_KEY`)

### 빌드

```bash
git clone https://github.com/gyulsbox/EUV_Idea_Variation_2_kr
cd EUV_Idea_Variation_2_kr
go build -o iv2loc ./cmd/iv2loc
```

### 모드 만들기

번역이 끝난 `catalog.json`이 저장소에 들어 있어서, **API 키 없이 바로 만들 수 있어요.**

```bash
./iv2loc build \
  --src "C:/Program Files (x86)/Steam/steamapps/workshop/content/3450310/3599735023" \
  --out "$USERPROFILE/Documents/Paradox Interactive/Europa Universalis V/mod/iv2_korean" \
  --catalog catalog.json
```

빌드가 끝나면 `validate`가 자동으로 한 번 더 돌아요. 뭔가 잘못되면 0이 아닌 코드로
종료하니까 조용히 깨진 팩이 나올 일은 없습니다.

### 그 외 명령

```bash
./iv2loc extract  --src <IV2>                        # 키가 몇 개고 실제 번역 대상이 몇 개인지
./iv2loc validate --src <IV2> --out <팩>             # 토큰 무결성 + 본체 키 침범 검사
./iv2loc diff     --src <IV2> --catalog catalog.json # IV2 업데이트 후 뭐가 바뀌었는지
./iv2loc polish   --catalog catalog.json             # 한국어 조사 기계적 교정
./iv2loc keys     --src <트리> --out keys.txt        # 로컬 트리를 키 이름만으로 축약
```

각 명령에 `-h`를 붙이면 플래그를 볼 수 있어요.

### 번역을 직접 돌리기

새로 생긴 문자열만 채우거나, 처음부터 다시 번역하고 싶을 때요.

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./iv2loc translate --catalog catalog.json --glossary glossary.json
```

기본 모델은 `claude-haiku-4-5`입니다. 2,766개 전부 돌려도 1달러가 안 들어요.
`--model`로 다른 모델을 쓸 수 있고, `--limit 50`으로 일부만 돌려서 품질을 먼저
확인해볼 수도 있습니다. `--dry-run`은 API를 부르지 않고 프롬프트만 보여줘요.

### IV2가 업데이트되면

세 줄이면 따라잡습니다.

```bash
./iv2loc diff      --src <새 IV2> --catalog catalog.json --out <팩> --update
./iv2loc translate --catalog catalog.json --glossary glossary.json
./iv2loc build     --src <새 IV2> --out <팩> --catalog catalog.json
```

번역 단위를 키가 아니라 **영어 원문 기준**으로 관리해요. 원문이 그대로면 번역도 그대로
남으니까, 업데이트할 때 실제로 바뀐 만큼만 다시 번역하면 됩니다.
실제로 문구 1개 수정 + 2개 추가 + 1개 삭제로 테스트해보니 2,766개가 아니라
**3개**만 다시 번역하면 됐어요.

---

## 번역이 어색할 때

- **용어가 왔다갔다 한다면** → `glossary.json`에 추가하고
  `./iv2loc translate --catalog catalog.json --glossary glossary.json --retranslate`
- **문장 하나가 어색하다면** → `catalog.json`에서 그 유닛의 `korean`을 고치고 다시 `build`

`catalog.json`의 유닛은 이렇게 생겼어요. `members`를 보면 그 문장이 게임 안에서
몇 개 키에 쓰이는지 알 수 있습니다.

```json
{ "template": "Level ⟦N1⟧", "korean": "레벨 ⟦N1⟧",
  "rep": "iv2_level_1", "class": "prose", "members": 7 }
```

---

## 어떻게 동작하나

**⟦T1⟧ / ⟦N1⟧ 자리표시자.** 파라독스 마크업(`$VAR$`, `[Scope.Func]`, `#bold`, `£icon£`,
`@key!`, `\n`)이랑 숫자를 번역기에 넘기기 전에 번호 붙은 자리표시자로 빼둡니다.
마크업이 깨질 일이 없어지고, 마크업이나 숫자만 다른 문자열들이 하나로 합쳐져요.
**키 50,052개가 번역 단위 2,766개로 줄어듭니다 (18배).** 번호를 붙여놨으니 한국어
어순에 맞게 위치를 옮겨도 괜찮아요.

**토큰 개수 검사.** 원문과 번역문의 마크업 토큰 개수가 같은지 봅니다. 순서는 안 봐요
(한국어는 어순이 달라지니까요). 안 맞으면 그 키는 영어 원문으로 되돌립니다.
그래서 번역이 좀 틀려도 게임이 죽지는 않아요.

**조사.** `⟦T1⟧을(를)`처럼 자리표시자 뒤에 오는 조사는 값을 모르니까 짝 형태를 씁니다.
그런데 `build`가 숫자를 넣는 순간엔 값을 알게 되죠. 그래서 거기서 한 번 더 풀어요
(`⟦N1⟧으로(로)` → `4로`). 최종 팩에 남아 있는 짝 형태는 전부 게임이 런타임에 채우는
자리 뒤에 있는 것들입니다.

**출력 구조.** IV2의 파일 구조를 그대로 따라가고, `main_menu/localization/korean/` 아래에
`0_` 접두사를 붙여서 씁니다. EU5는 로컬 파일을 역알파벳순으로 읽고 먼저 정의된 키가
이기기 때문에, `0_`을 붙이면 우리가 제일 뒤로 갑니다. 다른 데서 이미 그 키를 한국어로
정의해뒀다면 그쪽이 이기게 되는 거죠. 우리는 IV2에 없는 한국어 레이어를 채우는 거지
키를 가져오는 게 아니니까 이게 맞아요.

**UTF-8 BOM.** BOM이 없으면 EU5가 파일을 그냥 무시합니다. 에러도 안 뜨고요.
그래서 출력할 때 항상 붙이고, 테스트에서 바이트 단위로 확인합니다.

**IV2가 정의한 키만 건드립니다.** 로컬 파일이 게임 본체나 다른 모드의 키를 덮으면
충돌이 나거든요. `iv2loc`은 IV2의 영어 파일에서 뽑은 키만 출력하기 때문에 구조적으로
그럴 수가 없고, 혹시 모르니 `validate`가 한 번 더 확인합니다.

---

## validate가 보는 것

| 규칙 | 심각도 | 내용 |
|---|---|---|
| `token-mismatch` | error | 마크업이 사라지거나 늘거나 바뀜 |
| `leaked-placeholder` | error | `⟦T1⟧`이 출력까지 남아 있음 |
| `empty-translation` | error | 내용이 있던 문자열이 비어버림 |
| `do-not-translate` | error | 게임이 값으로 읽는 것(색상명 등)을 번역함 |
| `unknown-key` | error | IV2에 없는 키를 만듦 |
| `shadows-baseline` | error | 본체나 다른 모드의 키를 덮음 |
| `layer-mismatch` | error | IV2와 다른 레이어에 키를 넣음 |
| `missing-bom` | error | 게임이 파일을 무시하게 됨 |
| `bad-header` / `bad-filename` | error | 로드되지 않는 형식 |
| `replace-dir` | error | `replace/`를 씀 (설계상 필요 없어야 정상) |
| `duplicate-key` / `untranslated` / `malformed-source` | warn | |

규칙마다 일부러 어긋난 값을 넣어서 잡히는지 확인하는 테스트가 붙어 있어요.

### 본체 키 충돌 검사

비교할 트리를 주지 않으면 `NOT CHECKED`로 나옵니다. 확인 안 한 걸 통과라고 하면
안 되니까요. 확인하고 싶으면 본체 경로를 넘겨주세요. EU5는 로컬을
`game/{dlc,in_game,loading_screen,main_menu}/localization/`으로 나눠놓는데,
레이어 위쪽 디렉터리를 지정하면 알아서 전부 훑습니다.

```bash
./iv2loc validate --src <IV2> --out <팩> --baseline "<EU5>/game"
```

트리가 너무 크면 키 이름만 뽑아서 넘겨도 돼요.

```bash
./iv2loc keys     --src "<EU5>/game" --out basegame.keys
./iv2loc validate --src <IV2> --out <팩> --baseline-keys basegame.keys
```

참고로 미리 확인해보니 IV2 로컬에는 본체 키가 없어요. 네임스페이스가 없는 키 6,249개 중
5,953개는 값이 iv2를 참조하고, 남은 296개도 전부 `iv_` 접두사거나 Idea Variation 관련
이름이었습니다. 그래서 이 검사는 사실상 찾을 게 없고, 빌드하는 데 꼭 필요하지도 않아요.

---

## 개발

```bash
go test ./...
```

| 패키지 | 하는 일 |
|---|---|
| `internal/paradox` | 파라독스 yml 파서, 마크업 토큰 스캐너 |
| `internal/inventory` | 모드 순회, 값 분류, 번역 단위 그룹핑 |
| `internal/validate` | 불변식 검사, 영어 폴백 |
| `internal/korean` | 조사 기계적 교정 |
| `internal/translate` | Claude API 호출, 용어집, 재시도 |
| `internal/build` | 팩 생성, 메타데이터 |
| `internal/diff` | 업데이트 비교, 카탈로그 병합 |

파라독스 `.yml`은 이름만 yml이고 YAML이 아니에요. 값은 항상 따옴표로 감싸이고, 키에
버전 번호가 붙고, 값 안의 `#bold` 같은 포맷 토큰이 YAML 주석 문법이랑 겹칩니다.
표준 YAML 파서를 쓰면 망가지니까 직접 파싱합니다.

테스트 픽스처(`internal/*/testdata`)는 실제 IV2에서 잘라왔고, IV2가 실제로 가지고 있는
특이 케이스들도 같이 들어 있어요 — BOM 없는 파일, 중복 키, 닫는 따옴표가 빠진 값 8개요.
