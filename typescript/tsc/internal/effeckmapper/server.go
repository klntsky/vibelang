package effeckmapper

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/contentmapper"
	"github.com/microsoft/TypeScript/tsc/internal/ipc"
	"github.com/microsoft/TypeScript/tsc/internal/json"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
)

// DiagnosticSource prefixes the code of every diagnostic the mapper reports,
// so EK9001 reads like the fork's own EK diagnostics.
const DiagnosticSource = "EK"

// VirtualExtension is the extension of every projected file: TypeScript
// checks greet.effeck as greet.effeck.ts.
const VirtualExtension = ".ts"

// Serve answers TypeScript's content mapper requests on rwc until the
// connection closes or ctx ends. TypeScript sends requests concurrently, and
// the connection handles each in its own goroutine; Project shares no state
// between calls, so transforms run in parallel.
func Serve(ctx context.Context, rwc io.ReadWriteCloser) error {
	return ipc.NewAsyncConn(rwc, Handler{}).Run(ctx)
}

// Handler implements the content mapper protocol: initialize, openProject,
// transform and closeProject. The protocol is driven by TypeScript alone,
// and the mapper keeps no per-project state: a projection depends only on
// its file's text and context (FileContextOf), which reads package.json
// files that openProject reports as watched.
type Handler struct{}

// HandleRequest answers one protocol request.
func (Handler) HandleRequest(ctx context.Context, method string, params json.Value) (any, error) {
	switch method {
	case contentmapper.MethodInitialize:
		return initialize(params)
	case contentmapper.MethodOpenProject:
		return openProject(params)
	case contentmapper.MethodCloseProject:
		return nil, nil
	case contentmapper.MethodTransform:
		return transform(params)
	default:
		return nil, fmt.Errorf("effeck-mapper: unknown method %q", method)
	}
}

// HandleNotification ignores notifications; the protocol defines none.
func (Handler) HandleNotification(ctx context.Context, method string, params json.Value) error {
	return nil
}

// initialize selects UTF-8 positions, which are Go's string offsets, so
// the projector's byte offsets go on the wire unconverted.
func initialize(params json.Value) (any, error) {
	var p contentmapper.InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if !slices.Contains(p.PositionEncodings, contentmapper.PositionEncodingUTF8) {
		return nil, fmt.Errorf("effeck-mapper: the host does not accept utf-8 positions (offered %v)", p.PositionEncodings)
	}
	return contentmapper.InitializeResult{
		PositionEncoding: contentmapper.PositionEncodingUTF8,
		DiagnosticSource: DiagnosticSource,
	}, nil
}

// openProject reports the package.json files whose names enter the
// project's module keys as watched files (the manifest declares
// dynamicConfig), with an identity of their names, so a renamed package
// invalidates every projection of the project. It also reports EK9002 when
// the project installs another version of effect or of an @effect package
// than the forms table was generated from (versionMismatch).
func openProject(params json.Value) (any, error) {
	var p contentmapper.OpenProjectParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	files, identity := projectPackageFiles(p.ConfigFileName)
	result := contentmapper.OpenProjectResult{ConfigIdentity: identity, WatchedFiles: files}
	if table, err := DefaultTable(); err == nil && p.ConfigFileName != "" {
		dir := path.Dir(filepath.ToSlash(p.ConfigFileName))
		if message := versionMismatch(table.Header, dir, os.ReadFile); message != "" {
			// A project-level diagnostic, on the mapper's tsconfig entry.
			result.OptionDiagnostics = append(result.OptionDiagnostics, contentmapper.OptionDiagnosticResult{
				Path:        []json.Value{},
				MessageText: message,
				Code:        CodeVersionMismatch,
			})
		}
	}
	return result, nil
}

// transform projects one file from its content and its context; the project
// handle does not change the result.
func transform(params json.Value) (any, error) {
	var p contentmapper.TransformParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return TransformResult(Project(p.Content, FileContextOf(p.FileName)))
}

// TransformResult encodes projection as the protocol's transform result.
func TransformResult(projection Projection) (contentmapper.TransformResult, error) {
	mappings, err := spanmap.New(projection.Segments).Marshal()
	if err != nil {
		return contentmapper.TransformResult{}, err
	}
	result := contentmapper.TransformResult{
		MappedOutput: contentmapper.MappedOutput{
			Text:      projection.Text,
			Extension: VirtualExtension,
			Mappings:  json.Value(mappings),
		},
	}
	for _, d := range projection.Diagnostics {
		result.Diagnostics = append(result.Diagnostics, contentmapper.Diagnostic{
			MessageText: d.Message,
			Start:       d.Start,
			Length:      d.Length,
			Code:        d.Code,
		})
	}
	return result, nil
}
