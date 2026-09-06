package tml_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/xml-validator/validator"

	"github.com/wow-look-at-my/tml/syntax"
)

const schemaPath = "tml.schema.xsd"

// negativeCorpus holds the documents the schema must REJECT, so the walk that requires every document to validate skips
// it. A file anywhere else in the tree is a real document and has to pass.
const negativeCorpus = "testdata/schema/invalid"

// rejection reads the reason a fixture states it must be rejected for. A fixture that states none fails: a document that
// is merely rejected proves nothing, because the wrong rule rejects it just as loudly as the right one.
var rejection = regexp.MustCompile(`(?m)^<!-- rejects: (.+) -->$`)

func schemaBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	return data
}

func tmlFiles(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".tml" {
			found = append(found, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, found)
	return found
}

// The schema must never reject a document TML accepts. syntax.Parse is the lower bound on that: it decides the shape the
// schema describes, so a file it accepts must validate. The bound is not tight in one direction -- a style attribute's
// vocabulary is checked when the sheet resolves, well after Parse -- so a fixture the schema rejects and Parse accepts
// lives in the negative corpus, which this walk skips.
func TestEveryDocumentTheParserAcceptsValidates(t *testing.T) {
	xsd := schemaBytes(t)
	checked := 0
	for _, path := range tmlFiles(t, ".") {
		if strings.HasPrefix(filepath.ToSlash(path), negativeCorpus) {
			continue
		}
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		if _, err := syntax.Parse(path, src); err != nil {
			continue
		}
		assert.NoError(t, validator.ValidateWithSchemaBytes(src, xsd), "%s parses, so the schema must accept it", path)
		checked++
	}
	assert.Positive(t, checked)
}

func TestTheSchemaRejectsEachFixtureForItsOwnReason(t *testing.T) {
	xsd := schemaBytes(t)
	for _, path := range tmlFiles(t, negativeCorpus) {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".tml"), func(t *testing.T) {
			src, err := os.ReadFile(path)
			require.NoError(t, err)

			stated := rejection.FindSubmatch(src)
			require.NotNil(t, stated, "%s has no <!-- rejects: ... --> comment naming why it must fail", path)

			err = validator.ValidateWithSchemaBytes(src, xsd)
			require.Error(t, err, "%s must not validate", path)
			assert.ErrorContains(t, err, string(stated[1]))
		})
	}
}
