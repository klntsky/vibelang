package effeckmapper

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/contentmapper"
	"github.com/microsoft/TypeScript/tsc/internal/locale"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"gotest.tools/v3/assert"
)

// TestHostTransform drives the mapper with TypeScript's own content mapper
// host over an in-memory connection: initialize, openProject and transform
// as tsc sends them, then the validation and parse tsc applies to the result.
func TestHostTransform(t *testing.T) {
	t.Parallel()
	spawner := contentmapper.SpawnerFunc(func(command []string, dir string, stderr io.Writer) (io.ReadWriteCloser, error) {
		client, server := net.Pipe()
		go func() { _ = Serve(context.Background(), server) }()
		return client, nil
	})
	host := contentmapper.NewHost(t.Context(), spawner, locale.Default)
	defer host.Close()
	mapper := &contentmapper.Mapper{
		Definition: contentmapper.Definition{Package: "effeck-ts-mapper", Extensions: []string{".effeck"}},
		Manifest:   contentmapper.Manifest{Name: "effeck-ts-mapper", Version: "0.0.1", Exec: []string{"./bin/effeck-mapper"}, DynamicConfig: true},
	}
	project := host.Project(contentmapper.ProjectSpec{ConfigFileName: "/p/tsconfig.json", Mappers: []*contentmapper.Mapper{mapper}})
	defer project.Close()

	fileName := tspath.RootedFilePathFromAbsolute("/p/greet.effeck")
	content := "error EmptyName { name: string }\nexport function greet(name: string) {\n  if (name === \"\") throw new EmptyName({ name })\n  return await.all [name]\n}\n"
	files, err := contentmapper.TransformAndParse(ast.SourceFileParseOptions{FileName: fileName}, content, mapper, project)
	assert.NilError(t, err)

	file := files.Canonical
	assert.Equal(t, file.VirtualFileName(), fileName.AppendSuffix(".ts"))
	assert.Equal(t, file.OriginalText(), content)
	assert.Equal(t, file.Text(), Project(content, FileContextOf(fileName.AsString())).Text)
	// The mapper's EK9001 on the await operation arrives in original coordinates. Filter
	// by source: ParseResult parses the virtual text under the original
	// name, greet.effeck, so in the fork the parser reads it with Effeck
	// syntax and reports none of the TS1434 errors stock tsc reports there.
	var mapperDiagnostics []*ast.Diagnostic
	for _, d := range file.Diagnostics() {
		if d.Source() == DiagnosticSource {
			mapperDiagnostics = append(mapperDiagnostics, d)
		}
	}
	assert.Equal(t, len(mapperDiagnostics), 1)
	assert.Equal(t, mapperDiagnostics[0].Code(), int32(CodeUnprojectedForm))
	assert.Equal(t, content[mapperDiagnostics[0].Pos():mapperDiagnostics[0].End()], "await")
}

// TestInitializeRequiresUTF8 checks that the mapper refuses a host that does
// not accept its byte offsets.
func TestInitializeRequiresUTF8(t *testing.T) {
	t.Parallel()
	_, err := Handler{}.HandleRequest(t.Context(), contentmapper.MethodInitialize, []byte(`{"positionEncodings":["utf-16"]}`))
	assert.ErrorContains(t, err, "utf-8")
	result, err := Handler{}.HandleRequest(t.Context(), contentmapper.MethodInitialize, []byte(`{"positionEncodings":["utf-8","utf-16"]}`))
	assert.NilError(t, err)
	assert.Equal(t, result.(contentmapper.InitializeResult).PositionEncoding, contentmapper.PositionEncodingUTF8)
}
