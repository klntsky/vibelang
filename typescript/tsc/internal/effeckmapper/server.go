package effeckmapper

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

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
	return ipc.NewAsyncConn(rwc, &Handler{}).Run(ctx)
}

// Handler implements the content mapper protocol: initialize, openProject,
// transform and closeProject. The protocol is driven by TypeScript alone. A
// projection depends only on its file's text and context (FileContextOf),
// which reads package.json files that openProject reports as watched, and
// on the options of the mapper's tsconfig entry, which openProject records
// per project handle.
type Handler struct {
	mu       sync.Mutex
	projects map[string]Options
}

// Options are the mapper entry's options in a project's tsconfig:
//
//	"contentMappers": [{ "package": "effeck-ts-mapper", "extensions": [".effeck"], "options": { "tracing": "all" } }]
//
// TypeScript folds them into the transform identity, so changing one
// projects every file again.
type Options struct {
	// Tracing is the fork's compilerOptions.effeck.tracing (D34):
	// "exported", "all", or "" for none. Only the executable rendering
	// reads it; spans change no type.
	Tracing string `json:"tracing,omitempty"`
}

// HandleRequest answers one protocol request.
func (h *Handler) HandleRequest(ctx context.Context, method string, params json.Value) (any, error) {
	switch method {
	case contentmapper.MethodInitialize:
		return initialize(params)
	case contentmapper.MethodOpenProject:
		return h.openProject(params)
	case contentmapper.MethodCloseProject:
		var p contentmapper.CloseProjectParams
		if err := json.Unmarshal(params, &p); err == nil {
			h.mu.Lock()
			delete(h.projects, p.ProjectHandle)
			h.mu.Unlock()
		}
		return nil, nil
	case contentmapper.MethodTransform:
		return h.transform(params)
	default:
		return nil, fmt.Errorf("effeck-mapper: unknown method %q", method)
	}
}

// HandleNotification ignores notifications; the protocol defines none.
func (*Handler) HandleNotification(ctx context.Context, method string, params json.Value) error {
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
// invalidates every projection of the project. It also reports EK9004 when
// the project installs another version of effect or of an @effect package
// than the forms table was generated from (versionMismatch).
func (h *Handler) openProject(params json.Value) (any, error) {
	var p contentmapper.OpenProjectParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	files, identity := projectPackageFiles(p.ConfigFileName)
	result := contentmapper.OpenProjectResult{ConfigIdentity: identity, WatchedFiles: files}
	var options Options
	if len(p.Options) > 0 && json.Unmarshal(p.Options, &options) != nil {
		result.OptionDiagnostics = append(result.OptionDiagnostics, contentmapper.OptionDiagnosticResult{
			Path: []json.Value{}, MessageText: "effeck-ts-mapper's options are an object: { \"tracing\": \"exported\" | \"all\" }.", Code: CodeInvalidOption,
		})
	}
	switch options.Tracing {
	case "", "exported", "all":
	default:
		result.OptionDiagnostics = append(result.OptionDiagnostics, contentmapper.OptionDiagnosticResult{
			Path:        []json.Value{json.Value(`"tracing"`)},
			MessageText: "tracing is \"exported\" or \"all\" (D34), not " + strconv.Quote(options.Tracing) + ".",
			Code:        CodeInvalidOption,
		})
		options.Tracing = ""
	}
	h.mu.Lock()
	if h.projects == nil {
		h.projects = map[string]Options{}
	}
	h.projects[p.ProjectHandle] = options
	h.mu.Unlock()
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

// transform projects one file from its content and its context, with the
// options its project's tsconfig entry gives.
func (h *Handler) transform(params json.Value) (any, error) {
	var p contentmapper.TransformParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	ctx := FileContextOf(p.FileName)
	h.mu.Lock()
	ctx.Tracing = h.projects[p.ProjectHandle].Tracing
	h.mu.Unlock()
	return TransformResult(Project(p.Content, ctx))
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
