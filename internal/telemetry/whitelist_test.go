package telemetry_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fields that must never be sent to telemetry. This list matches the deny
// list documented in TELEMETRY.md and exists as a machine-checkable guard so
// accidentally adding a `"username": user.Name` entry to a telemetry.Log
// call fails the build rather than shipping user data off-device.
var bannedAttributeKeys = map[string]struct{}{
	"password":        {},
	"master_password": {},
	"username":        {},
	"riotId":          {},
	"riot_id":         {},
	"puuid":           {},
	"displayName":     {},
	"display_name":    {},
	"notes":           {},
	"tags":            {},
	// Recovery material must never leave the device. Flagged even though
	// the crypto package only ever holds these transiently, because a
	// future refactor could accidentally thread them into a Log call.
	"recovery_phrase": {},
	"recovery_hash":   {},
	"vault_key":       {},
	"vault_path":      {},
}

// TestTelemetryAttributesAreNotVaultFields walks every Go file in the
// repository that imports the telemetry package and inspects the AST of each
// telemetry.Log* call. Any map-literal attribute key matching the banned list
// fails the test.
//
// The test is deliberately loud about false positives — if a refactor
// introduces a collision (e.g. a sanitized "username" that is, in fact, the
// OS username and safe to log) the right fix is to rename the key to
// something unambiguous like "os_user" rather than silence the check.
func TestTelemetryAttributesAreNotVaultFields(t *testing.T) {
	root := projectRoot(t)

	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip vendored / generated / test-fixture dirs.
			name := info.Name()
			if name == "node_modules" || name == "vendor" || name == "frontend" || name == "build" ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Don't scan this test file — it contains the banned strings by design.
		if strings.HasSuffix(path, "whitelist_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return parseErr
		}

		// Only scan files that actually import the telemetry package.
		if !importsTelemetry(node) {
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !isTelemetryLogCall(call) {
				return true
			}
			for _, arg := range call.Args {
				cl, ok := arg.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					lit, ok := kv.Key.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					// Trim surrounding quotes.
					key := strings.Trim(lit.Value, `"`+"`")
					if _, banned := bannedAttributeKeys[key]; banned {
						pos := fset.Position(lit.Pos())
						violations = append(violations,
							pos.Filename+":"+itoa(pos.Line)+" — banned telemetry key "+lit.Value)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk failed: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("telemetry attribute allowlist violated — these keys may leak vault data:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func importsTelemetry(f *ast.File) bool {
	for _, imp := range f.Imports {
		// Path literal includes quotes.
		if strings.Contains(imp.Path.Value, "OpenSmurfManager/internal/telemetry") {
			return true
		}
	}
	return false
}

// isTelemetryLogCall returns true for calls shaped like `telemetry.Log...(...)`.
// Works for telemetry.Log, telemetry.LogInfo, telemetry.LogError, etc.
func isTelemetryLogCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "telemetry" {
		return false
	}
	return strings.HasPrefix(sel.Sel.Name, "Log")
}

// projectRoot walks up from the test file's dir to find the repo root
// (marked by go.mod). Used so the test works whether it's run from the
// package dir or the project root.
func projectRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", cwd)
		}
		dir = parent
	}
}

// itoa avoids pulling in strconv for this tiny use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
