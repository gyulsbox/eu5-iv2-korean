package translate

import "testing"

func TestFixParticlesKeepsPairAfterPlaceholder(t *testing.T) {
	// This is the one case the paired form exists for: the substituted value
	// is unknown, so its final consonant is unknowable.
	cases := map[string]string{
		"⟦T1⟧을(를) 선택했습니다":    "⟦T1⟧을(를) 선택했습니다",
		"⟦T1⟧를(을) 선택했습니다":    "⟦T1⟧을(를) 선택했습니다",
		"⟦T2⟧이(가) 필요합니다":     "⟦T2⟧이(가) 필요합니다",
		"⟦N1⟧은(는) 환불되지 않습니다": "⟦N1⟧은(는) 환불되지 않습니다",
		"⟦T1⟧과(와) 함께":        "⟦T1⟧와(과) 함께",
		"⟦T1⟧과(과) 모든 업그레이드":  "⟦T1⟧와(과) 모든 업그레이드",
		"⟦T1⟧으로(로) 잠금 해제":    "⟦T1⟧으로(로) 잠금 해제",
		"⟦T1⟧로(으로) 잠금 해제":    "⟦T1⟧으로(로) 잠금 해제",
	}
	for in, want := range cases {
		if got := FixParticles(in); got != want {
			t.Errorf("FixParticles(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFixParticlesResolvesAfterOrdinaryWords(t *testing.T) {
	// Here the consonant is known, so writing the pair is just sloppy.
	cases := map[string]string{
		// 탭 ends in ㅂ -> 을
		"선택 서브탭을(를) 열어": "선택 서브탭을 열어",
		// 이념 ends in ㅁ -> 을
		"국가 이념을(를) 관리": "국가 이념을 관리",
		// 슬롯 ends in ㅅ -> 을
		"슬롯을(를) 잠금 해제": "슬롯을 잠금 해제",
		// 명분 ends in ㄴ -> 을
		"명분을(를) 생성": "명분을 생성",
		// 그룹 ends in ㅂ -> 이
		"이념 그룹이(가) 있습니다": "이념 그룹이 있습니다",
		// 개 is open -> 가
		"두 개가(이) 있습니다": "두 개가 있습니다",
		// 나라 is open -> 는
		"나라는(은) 부유합니다": "나라는 부유합니다",
		// 국가 is open -> 와
		"국가와(과) 도시": "국가와 도시",
		// 도시 is open -> 와
		"도시과(와) 마을": "도시와 마을",
	}
	for in, want := range cases {
		if got := FixParticles(in); got != want {
			t.Errorf("FixParticles(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFixParticlesHandlesEuroRo(t *testing.T) {
	cases := map[string]string{
		// 전환 ends in ㄴ -> 으로
		"산업 사회로의 전환으로(로) 표시": "산업 사회로의 전환으로 표시",
		// 이유 is open -> 로
		"이유으로(로) 인해": "이유로 인해",
		// ㄹ is the exception: 서울로, never 서울으로
		"서울으로(로) 이동": "서울로 이동",
		"수도로(으로) 이동": "수도로 이동",
	}
	for in, want := range cases {
		if got := FixParticles(in); got != want {
			t.Errorf("FixParticles(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFixParticlesAfterDigits(t *testing.T) {
	// A particle after a numeral agrees with how the numeral is spoken.
	cases := map[string]string{
		"레벨 1을(를) 선택": "레벨 1을 선택", // 일 ends in ㄹ
		"레벨 2을(를) 선택": "레벨 2를 선택", // 이 is open
		"레벨 3을(를) 선택": "레벨 3을 선택", // 삼 ends in ㅁ
		"레벨 4을(를) 선택": "레벨 4를 선택", // 사 is open
		"레벨 6을(를) 선택": "레벨 6을 선택", // 육 ends in ㄱ
	}
	for in, want := range cases {
		if got := FixParticles(in); got != want {
			t.Errorf("FixParticles(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFixParticlesLeavesUnknowableAlone(t *testing.T) {
	// After Latin text there is no basis to choose, and guessing would be
	// worse than the paired form.
	cases := []string{
		"Ideagroup을(를) 선택",
		"OK을(를) 누르세요",
	}
	for _, in := range cases {
		if got := FixParticles(in); got != in {
			t.Errorf("FixParticles(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestFixParticlesLeavesOrdinaryTextAlone(t *testing.T) {
	cases := []string{
		"국가 이념을 관리할 수 있습니다.",
		"⟦T1⟧ ⟦T2⟧",
		"",
		"Innovativeness Ideas",
		// A parenthesis that is not a particle pair must survive.
		"이념 그룹(최대 4개)을 선택",
	}
	for _, in := range cases {
		if got := FixParticles(in); got != in {
			t.Errorf("FixParticles(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestFixParticlesHandlesSeveralInOneString(t *testing.T) {
	in := "이 탭을(를) 통해 ⟦T1⟧을(를) 관리하고 국가 이념을(를) 선택합니다."
	want := "이 탭을 통해 ⟦T1⟧을(를) 관리하고 국가 이념을 선택합니다."
	if got := FixParticles(in); got != want {
		t.Errorf("FixParticles(%q)\n got %q\nwant %q", in, got, want)
	}
}

func TestFixParticlesNeverTouchesPlaceholders(t *testing.T) {
	// The invariant everything else rests on must survive this pass.
	in := "⟦T1⟧을(를) ⟦N1⟧과(와) ⟦T2⟧으로(로)"
	got := FixParticles(in)
	for _, ph := range []string{"⟦T1⟧", "⟦N1⟧", "⟦T2⟧"} {
		if !contains(got, ph) {
			t.Errorf("FixParticles dropped %s: %q", ph, got)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
