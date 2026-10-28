package auth

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCodeVerifier_MeetsPKCERequirements(t *testing.T) {
	verifier, err := newCodeVerifier()
	require.NoError(t, err)

	// RFC 7636: code_verifier は 43〜128 文字の unreserved 文字列。
	assert.GreaterOrEqual(t, len(verifier), 43)
	assert.LessOrEqual(t, len(verifier), 128)
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9._~-]+$`), verifier)
}

func TestNewCodeVerifier_IsUnique(t *testing.T) {
	first, err := newCodeVerifier()
	require.NoError(t, err)
	second, err := newCodeVerifier()
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

// RFC 7636 Appendix B のテストベクタ。
func TestCodeChallenge_MatchesRFC7636Vector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", codeChallenge(verifier))
}
