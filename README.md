# iv2loc

Europa Universalis V 모드 [The Idea Variation 2][iv2](워크샵 ID `3599735023`)의
한국어 번역 모드와, 그걸 만들고 유지하는 도구.

[iv2]: https://steamcommunity.com/sharedfiles/filedetails/?id=3599735023

기존 커뮤니티 한글패치는 게임을 크래시시켰다. 원인은 번역이 아니라 포장이었다.
IV2와 무관한 파일 11개(`cw_settings`, `languages`, `jomini/multiplayer_gui`)를
같이 던져놔서 게임 본체 로컬을 덮었고, 키 충돌 153건이 났다.

**그래서 이 프로젝트의 제1원칙: IV2가 정의한 키만 번역한다.**
`iv2loc`은 IV2의 영어 파일에서 뽑은 키만 출력하므로 구조적으로 본체를 건드릴 수 없다.

현재 상태: **번역 완료.** 유니크 문자열 2,766개 전부 번역 → 실제 키 10,513개 한국어화.

---

## 설치 (플레이만 할 경우)

1. 빌드된 팩(`iv2_korean` 폴더)을 아래 경로에 넣는다.

   ```
   %USERPROFILE%\Documents\Paradox Interactive\Europa Universalis V\mod\iv2_korean\
   ```

   폴더 경로에 **한글이 들어가면 안 된다.** 게임이 로드에 실패한다.

2. 런처에서 활성화하고, 로드 순서를 **IV2 뒤**에 둔다.
3. 게임 언어를 한국어로 설정한다.

빌드된 팩은 저장소에 없다(생성물이라 커밋하지 않음). 아래 방법으로 직접 빌드하거나,
Release에 올려둔 zip을 받으면 된다.

---

## 실행 방법

### 요구사항

- Go 1.24 이상
- IV2 모드 경로 (보통 `C:\Program Files (x86)\Steam\steamapps\workshop\content\3450310\3599735023`)
- 번역을 새로 돌릴 때만: Anthropic API 키 (`ANTHROPIC_API_KEY` 환경변수)

### 빌드

```bash
git clone https://github.com/gyulsbox/EUV_Idea_Variation_2_kr
cd EUV_Idea_Variation_2_kr
go build -o iv2loc ./cmd/iv2loc
```

### 모드 만들기

이미 번역된 `catalog.json`이 저장소에 있으므로, **API 키 없이 바로 만들 수 있다.**

```bash
./iv2loc build \
  --src "C:/Program Files (x86)/Steam/steamapps/workshop/content/3450310/3599735023" \
  --out "$USERPROFILE/Documents/Paradox Interactive/Europa Universalis V/mod/iv2_korean" \
  --catalog catalog.json
```

빌드가 끝나면 자동으로 `validate`가 돌고, 문제가 있으면 0이 아닌 코드로 종료한다.

### 그 외 명령

```bash
./iv2loc extract  --src <IV2>                        # 키가 몇 개인지, 실제 번역 대상이 몇 개인지
./iv2loc validate --src <IV2> --out <팩>             # 토큰 무결성 + 본체 키 침범 검사
./iv2loc diff     --src <IV2> --catalog catalog.json # IV2 업데이트 후 뭐가 바뀌었는지
./iv2loc polish   --catalog catalog.json             # 한국어 조사 기계적 교정
./iv2loc keys     --src <트리> --out keys.txt        # 로컬 트리를 키 이름만으로 축약
```

`-h`로 각 명령의 플래그를 볼 수 있다.

### 번역을 직접 돌리기

`catalog.json`을 비우고 다시 하거나, 새로 생긴 문자열만 채울 때:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./iv2loc translate --catalog catalog.json --glossary glossary.json
```

기본 모델은 `claude-haiku-4-5`. 전체 2,766개를 돌려도 1달러 미만이다.
`--model`로 바꿀 수 있고, `--limit 50`으로 품질을 먼저 표본 확인할 수 있다.
`--dry-run`은 API를 호출하지 않고 프롬프트만 보여준다.

### IV2가 업데이트되면

세 줄이면 따라잡는다.

```bash
./iv2loc diff      --src <새 IV2> --catalog catalog.json --out <팩> --update
./iv2loc translate --catalog catalog.json --glossary glossary.json
./iv2loc build     --src <새 IV2> --out <팩> --catalog catalog.json
```

번역 단위는 **키가 아니라 영어 원문**으로 색인된다. 원문이 안 바뀐 문자열은 번역을
그대로 유지하므로, 업데이트 비용이 전체가 아니라 실제로 바뀐 것만큼만 든다.
실측(문구 1개 수정 + 2개 추가 + 1개 삭제): **2,766개가 아니라 3개**만 다시 번역.

---

## 번역 고치기

- **용어가 들쭉날쭉하다** → `glossary.json`에 추가 후
  `./iv2loc translate --catalog catalog.json --glossary glossary.json --retranslate`
- **문장 하나가 어색하다** → `catalog.json`에서 해당 유닛의 `korean`을 직접 고치고 `build`

`catalog.json`의 각 유닛은 이렇게 생겼다. `members`가 그 수정이 몇 개 키에 퍼지는지 알려준다.

```json
{ "template": "Level ⟦N1⟧", "korean": "레벨 ⟦N1⟧",
  "rep": "iv2_level_1", "class": "prose", "members": 7 }
```

---

## 설계 요점

**⟦T1⟧ / ⟦N1⟧ 플레이스홀더.** 파라독스 마크업(`$VAR$`, `[Scope.Func]`, `#bold`, `£icon£`,
`@key!`, `\n`)과 숫자를 번역기에 넘기기 전에 번호 붙은 자리표시자로 들어낸다. 덕분에
마크업이 깨질 수가 없고, 마크업·숫자만 다른 문자열이 하나로 합쳐진다.
**키 50,052개 → 번역 단위 2,766개 (18배 축소).** 번호식이라 한국어 어순대로 재배치해도 된다.

**토큰 멀티셋 불변식.** 원문과 번역문의 마크업 토큰 개수가 같아야 한다. 순서는 안 본다
(한국어는 어순이 바뀌므로). 어긋나면 그 키는 **영어 원문으로 되돌린다.**
→ 번역이 틀리면 게임이 이상하게 읽힐 뿐, 죽지는 않는다.

**조사 처리.** `⟦T1⟧을(를)`처럼 자리표시자 뒤에 붙는 조사는 값을 모르니 짝 형태를 쓴다.
하지만 `build`가 숫자를 넣는 순간 값을 알게 되므로 거기서 다시 푼다
(`⟦N1⟧으로(로)` → `4로`). 최종 팩에 남은 짝 형태는 전부 엔진이 런타임에 채우는 자리 뒤다.

**출력.** IV2의 파일 구조를 그대로 미러링하고, `main_menu/localization/korean/` 아래에
`0_` 접두사로 쓴다. EU5는 로컬 파일을 역알파벳순으로 처리하고 먼저 정의된 키가 이기므로,
`0_`은 우리를 **꼴찌**로 만든다 — 누가 이미 그 키를 한국어로 정의했으면 그쪽이 이긴다.
우리는 IV2에 없는 언어 레이어를 채우는 거지 키를 뺏는 게 아니다.

**UTF-8 BOM 필수.** BOM 없으면 EU5가 파일을 조용히 무시한다. 에러도 안 난다.

---

## validate가 잡는 것

| 규칙 | 심각도 | 내용 |
|---|---|---|
| `token-mismatch` | error | 마크업 유실·추가·변조 |
| `leaked-placeholder` | error | `⟦T1⟧`이 출력까지 살아남음 |
| `empty-translation` | error | 비어있지 않은 원문이 빈 문자열로 |
| `do-not-translate` | error | 엔진이 읽는 값(색상명 등)을 번역함 |
| `unknown-key` | error | IV2에 없는 키를 만듦 |
| `shadows-baseline` | error | 본체·타 모드 키 재정의 |
| `layer-mismatch` | error | IV2와 다른 레이어에 키를 넣음 |
| `missing-bom` | error | 게임이 파일을 무시하게 됨 |
| `bad-header` / `bad-filename` | error | 로드되지 않는 형식 |
| `replace-dir` | error | `replace/` 사용 = 설계 이탈 신호 |
| `duplicate-key` / `untranslated` / `malformed-source` | warn | |

각 규칙마다 위반 케이스를 먹여서 잡히는지 확인하는 테스트가 있다.

**본체 키 충돌 검사는 기본적으로 `NOT CHECKED`로 나온다.** 비교할 트리를 안 주면
통과라고 말하지 않는다 — 없는 걸 통과로 처리하는 게 기존 팩이 한 실수다.
증명하려면 본체 경로를 준다(레이어 위 디렉터리를 지정. EU5는 로컬을
`game/{dlc,in_game,loading_screen,main_menu}/localization/`으로 쪼개 놓는데 전부 훑는다):

```bash
./iv2loc validate --src <IV2> --out <팩> --baseline "<EU5>/game"
```

트리가 너무 크면 키 이름만 뽑아서 줘도 된다:

```bash
./iv2loc keys     --src "<EU5>/game" --out basegame.keys
./iv2loc validate --src <IV2> --out <팩> --baseline-keys basegame.keys
```

참고로 측정해보니 IV2 로컬에 본체 키는 없다. 네임스페이스 없는 키 6,249개 중
5,953개는 값이 iv2를 참조하고, 나머지 296개도 전부 `iv_` 접두사거나 Idea Variation
개념 이름이다. 그래서 이 검사는 실질적으로 찾을 게 없고, 빌드 전제조건도 아니다.

---

## 개발

```bash
go test ./...
```

| 패키지 | 역할 |
|---|---|
| `internal/paradox` | 파라독스 yml 파서 + 마크업 토큰 스캐너 |
| `internal/inventory` | 모드 순회, 값 분류, 번역 단위 그룹핑 |
| `internal/validate` | 불변식 검사, 영어 폴백 |
| `internal/korean` | 조사 기계적 교정 |
| `internal/translate` | Claude API 호출, 용어집, 재시도 |
| `internal/build` | 팩 생성, 메타데이터 |
| `internal/diff` | 업데이트 비교, 카탈로그 병합 |

파라독스 `.yml`은 **YAML이 아니다.** 값은 항상 따옴표로 감싸이고, 키에 버전 번호가
붙으며, 값 안의 `#bold` 같은 포맷 토큰이 YAML 주석 문법과 충돌한다. 표준 YAML 파서를
쓰면 망가진다.

테스트 픽스처(`internal/*/testdata`)는 실제 IV2에서 잘라냈고, IV2가 실제로 가진 결함을
포함한다 — BOM 없는 파일, 중복 키, 닫는 따옴표가 빠진 값 8개.
