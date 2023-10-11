package internal

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/inoxlang/inox/internal/config"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globalnames"
	"golang.org/x/exp/maps"

	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/containers"
	"github.com/inoxlang/inox/internal/globals/env_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/globals/inoxlsp_ns"
	"github.com/inoxlang/inox/internal/globals/strmanip_ns"
	"github.com/inoxlang/inox/internal/help"

	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"

	_ "github.com/inoxlang/inox/internal/local_db"
	_ "github.com/inoxlang/inox/internal/obs_db"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

var (
	DEFAULT_SCRIPT_LIMITS = []core.Limit{
		{Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},
		{Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},

		{Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
		{Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.TotalLimit, Value: 10_000},

		{Name: net_ns.HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
		{Name: net_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},
		{Name: net_ns.TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},

		{Name: s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 50},

		{Name: core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Kind: core.TotalLimit, Value: 5},
	}

	DEFAULT_REQUEST_HANDLING_LIMITS = []core.Limit{
		{Name: core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Kind: core.TotalLimit, Value: 2},
		{Name: core.EXECUTION_CPU_TIME_LIMIT_NAME, Kind: core.TotalLimit, Value: int64(25 * time.Millisecond)},
		{Name: core.EXECUTION_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: int64(5 * time.Second)},

		{Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000},
		{Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000},

		{Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 10},
		{Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.TotalLimit, Value: 100},

		{Name: net_ns.HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 1},
		{Name: net_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1},
		{Name: net_ns.TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1},

		{Name: s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 1},
	}

	DEFAULT_MAX_REQUEST_HANDLER_LIMITS = []core.Limit{
		{Name: core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Kind: core.TotalLimit, Value: 5},
		{Name: core.EXECUTION_CPU_TIME_LIMIT_NAME, Kind: core.TotalLimit, Value: int64(100 * time.Millisecond)},
		{Name: core.EXECUTION_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: int64(10 * time.Second)},

		{Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 10_000_000},
		{Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 10_000_000},

		{Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
		{Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.TotalLimit, Value: 1000},

		{Name: net_ns.HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 20},
		{Name: net_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 2},
		{Name: net_ns.TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 2},

		{Name: s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 10},
	}

	_ = []core.GoValue{
		(*html_ns.HTMLNode)(nil), (*core.GoFunction)(nil), (*http_ns.HttpServer)(nil), (*net_ns.TcpConn)(nil),
		(*net_ns.WebsocketConnection)(nil), (*http_ns.HttpRequest)(nil), (*http_ns.HttpResponseWriter)(nil),
		(*fs_ns.File)(nil),
	}
)

func init() {
	//set initial working directory on unix, on WASM it's done by the main package
	targetSpecificInit()
	registerHelp()

	inoxsh_ns.SetNewDefaultGlobalState(func(ctx *core.Context, envPattern *core.ObjectPattern, out io.Writer) *core.GlobalState {
		return utils.Must(NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
			EnvPattern: envPattern,
			Out:        out,
		}))
	})

	default_state.SetNewDefaultGlobalStateFn(NewDefaultGlobalState)
	default_state.SetNewDefaultContext(NewDefaultContext)
	default_state.SetDefaultScriptLimits(DEFAULT_SCRIPT_LIMITS)
	default_state.SetDefaultRequestHandlingLimits(DEFAULT_REQUEST_HANDLING_LIMITS)
	default_state.SetDefaultMaxRequestHandlerLimits(DEFAULT_MAX_REQUEST_HANDLER_LIMITS)
}

// NewDefaultGlobalState creates a new GlobalState with the default globals.
func NewDefaultGlobalState(ctx *core.Context, conf default_state.DefaultGlobalStateConfig) (*core.GlobalState, error) {
	logOut := conf.LogOut
	var logger zerolog.Logger
	if logOut == nil { //if there is not writer for logs we log to conf.Out
		logOut = conf.Out

		consoleLogger := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = logOut
			w.NoColor = !config.SHOULD_COLORIZE
			w.TimeFormat = "15:04:05"
			w.FieldsExclude = []string{"src"}
		})
		logger = zerolog.New(consoleLogger)
	} else {
		logger = zerolog.New(logOut)
	}

	logger = logger.With().Timestamp().Logger().Level(zerolog.InfoLevel)

	//create env namespace

	envNamespace, err := env_ns.NewEnvNamespace(ctx, conf.EnvPattern, conf.AllowMissingEnvVars)
	if err != nil {
		return nil, err
	}

	//create value for the preinit-data global
	var preinitFilesKeys []string
	var preinitDataValues []core.Serializable
	for _, preinitFile := range conf.PreinitFiles {
		preinitFilesKeys = append(preinitFilesKeys, preinitFile.Name)
		preinitDataValues = append(preinitDataValues, preinitFile.Parsed)
	}

	preinitData :=
		core.NewRecordFromKeyValLists([]string{"files"}, []core.Serializable{core.NewRecordFromKeyValLists(preinitFilesKeys, preinitDataValues)})

	//
	constants := map[string]core.Value{
		// constants
		core.INITIAL_WORKING_DIR_VARNAME:        core.INITIAL_WORKING_DIR_PATH,
		core.INITIAL_WORKING_DIR_PREFIX_VARNAME: core.INITIAL_WORKING_DIR_PATH_PATTERN,

		// namespaces
		globalnames.FS_NS:       fs_ns.NewFsNamespace(),
		globalnames.HTTP_NS:     http_ns.NewHttpNamespace(),
		globalnames.TCP_NS:      net_ns.NewTcpNamespace(),
		globalnames.DNS_NS:      net_ns.NewDNSnamespace(),
		globalnames.WS_NS:       net_ns.NewWebsocketNamespace(),
		globalnames.S3_NS:       s3_ns.NewS3namespace(),
		globalnames.CHROME_NS:   chrome_ns.NewChromeNamespace(),
		globalnames.ENV_NS:      envNamespace,
		globalnames.HTML_NS:     html_ns.NewHTMLNamespace(),
		globalnames.INOX_NS:     inox_ns.NewInoxNamespace(),
		globalnames.INOXSH_NS:   inoxsh_ns.NewInoxshNamespace(),
		globalnames.INOXLSP_NS:  inoxlsp_ns.NewInoxLspNamespace(),
		globalnames.STRMANIP_NS: strmanip_ns.NewStrManipNnamespace(),
		globalnames.RSA_NS:      newRSANamespace(),
		globalnames.INSECURE_NS: newInsecure(),

		globalnames.LS_FN: core.WrapGoFunction(fs_ns.ListFiles),

		// transaction
		globalnames.GET_CURRENT_TX_FN: core.ValOf(_get_current_tx),
		globalnames.START_TX_FN:       core.ValOf(core.StartNewTransaction),

		globalnames.ERROR_FN: core.ValOf(_Error),

		// resource
		globalnames.READ_FN: core.ValOf(_readResource),
		//globalnames.get:    core.ValOf(_getResource),
		globalnames.CREATE_FN: core.ValOf(_createResource),
		globalnames.UPDATE_FN: core.ValOf(_updateResource),
		globalnames.DELETE_FN: core.ValOf(_deleteResource),

		globalnames.SERVE_FN: core.ValOf(_serve),

		// events
		globalnames.EVENT_FN:     core.ValOf(_Event),
		globalnames.EVENT_SRC_FN: core.ValOf(core.NewEventSource),

		// watch
		globalnames.WATCH_RECEIVED_MESSAGES_FN: core.ValOf(core.WatchReceivedMessages),
		globalnames.VALUE_HISTORY_FN:           core.WrapGoFunction(core.NewValueHistory),
		globalnames.DYNIF_FN:                   core.WrapGoFunction(core.NewDynamicIf),
		globalnames.DYNCALL_FN:                 core.WrapGoFunction(core.NewDynamicCall),
		globalnames.GET_SYSTEM_GRAPH_FN:        core.WrapGoFunction(_get_system_graph),

		// send & receive values
		globalnames.SENDVAL_FN: core.ValOf(core.SendVal),

		// crypto
		globalnames.SHA256_FN:         core.ValOf(_sha256),
		globalnames.SHA384_FN:         core.ValOf(_sha384),
		globalnames.SHA512_FN:         core.ValOf(_sha512),
		globalnames.HASH_PASSWORD_FN:  core.ValOf(_hashPassword),
		globalnames.CHECK_PASSWORD_FN: core.ValOf(_checkPassword),
		globalnames.RAND_FN:           core.ValOf(_rand),

		//encodings
		globalnames.B64_FN:  core.ValOf(encodeBase64),
		globalnames.DB64_FN: core.ValOf(decodeBase64),

		globalnames.HEX_FN:   core.ValOf(encodeHex),
		globalnames.UNHEX_FN: core.ValOf(decodeHex),

		// conversion
		globalnames.TOSTR_FN:      core.ValOf(_tostr),
		globalnames.TORUNE_FN:     core.ValOf(_torune),
		globalnames.TOBYTE_FN:     core.ValOf(_tobyte),
		globalnames.TOFLOAT_FN:    core.ValOf(_tofloat),
		globalnames.TOINT_FN:      core.ValOf(_toint),
		globalnames.TORSTREAM_FN:  core.ValOf(_torstream),
		globalnames.TOJSON_FN:     core.ValOf(core.ToJSON),
		globalnames.TOPJSON_FN:    core.ValOf(core.ToPrettyJSON),
		globalnames.REPR_FN:       core.ValOf(_repr),
		globalnames.PARSE_REPR_FN: core.ValOf(_parse_repr),
		globalnames.PARSE_FN:      core.ValOf(_parse),
		globalnames.SPLIT_FN:      core.ValOf(_split),

		// time
		globalnames.AGO_FN:   core.ValOf(_ago),
		globalnames.NOW_FN:   core.ValOf(_now),
		globalnames.SLEEP_FN: core.ValOf(core.Sleep),

		// printing
		globalnames.LOGVALS_FN:       core.ValOf(_logvals),
		globalnames.LOG_FN:           core.ValOf(_log),
		globalnames.PRINT_FN:         core.ValOf(_print),
		globalnames.PRINTVALS_FN:     core.ValOf(_printvals),
		globalnames.FPRINT_FN:        core.ValOf(_fprint),
		globalnames.STRINGIFY_AST_FN: core.ValOf(_stringify_ast),
		globalnames.FMT_FN:           core.ValOf(core.Fmt),

		// bytes & string
		globalnames.MKBYTES_FN:       core.ValOf(_mkbytes),
		globalnames.RUNES_FN:         core.ValOf(_Runes),
		globalnames.BYTES_FN:         core.ValOf(_Bytes),
		globalnames.IS_RUNE_SPACE_FN: core.ValOf(_is_rune_space),
		globalnames.READER_FN:        core.ValOf(_Reader),
		globalnames.RINGBUFFER_FN:    core.ValOf(core.NewRingBuffer),

		// functional
		globalnames.IDENTITY_FN: core.WrapGoFunction(_idt),
		globalnames.MAP_FN:      core.WrapGoFunction(core.Map),
		globalnames.FILTER_FN:   core.WrapGoFunction(core.Filter),
		globalnames.SOME_FN:     core.WrapGoFunction(core.Some),
		globalnames.ALL_FN:      core.WrapGoFunction(core.All),
		globalnames.NONE_FN:     core.WrapGoFunction(core.None),
		globalnames.REPLACE_FN:  core.WrapGoFunction(_replace),
		globalnames.FIND_FN:     core.WrapGoFunction(_find),
		globalnames.SORT_FN:     core.WrapGoFunction(core.Sort),

		// concurrency & execution
		globalnames.LTHREADGROUP_FN: core.ValOf(core.NewLThreadGroup),
		globalnames.RUN_FN:          core.ValOf(_run),
		globalnames.EXEC_FN:         core.ValOf(_execute),
		globalnames.CANCEL_EXEC_FN:  core.ValOf(_cancel_exec),

		// integer
		globalnames.IS_EVEN_FN: core.ValOf(_is_even),
		globalnames.IS_ODD_FN:  core.ValOf(_is_odd),

		// protocol
		globalnames.SET_CLIENT_FOR_URL_FN:  core.ValOf(setClientForURL),
		globalnames.SET_CLIENT_FOR_HOST_FN: core.ValOf(setClientForHost),

		// other functions
		globalnames.ADD_CTX_DATA_FN: core.ValOf(_add_ctx_data),
		globalnames.CTX_DATA_FN:     core.ValOf(_ctx_data),
		globalnames.PROPNAMES_FN:    core.WrapGoFunction(_propnames),

		globalnames.ARRAY_FN: core.ValOf(core.NewArray),
		globalnames.LIST_FN:  core.ValOf(_List),

		globalnames.TYPEOF_FN:    core.ValOf(_typeof),
		globalnames.URL_OF_FN:    core.ValOf(_url_of),
		globalnames.LEN_FN:       core.ValOf(_len),
		globalnames.LEN_RANGE_FN: core.ValOf(_len_range),

		globalnames.SUM_OPTIONS_FN: core.ValOf(core.SumOptions),
		globalnames.MIME_FN:        core.ValOf(http_ns.Mime_),

		globalnames.COLOR_FN:    core.WrapGoFunction(_Color),
		globalnames.FILEMODE_FN: core.WrapGoFunction(core.FileModeFrom),

		globalnames.HELP_FN: core.ValOf(help.Help),
	}

	for k, v := range containers.NewContainersNamespace() {
		constants[k] = v
	}

	if conf.AbsoluteModulePath != "" {
		constants[default_state.MODULE_DIRPATH_GLOBAL_NAME] = core.DirPathFrom(filepath.Dir(conf.AbsoluteModulePath))
		constants[default_state.MODULE_FILEPATH_GLOBAL_NAME] = core.PathFrom(conf.AbsoluteModulePath)
	}

	baseGlobals := maps.Clone(constants)
	constants[default_state.PREINIT_DATA_GLOBAL_NAME] = preinitData

	symbolicBaseGlobals := map[string]symbolic.SymbolicValue{}
	{
		encountered := map[uintptr]symbolic.SymbolicValue{}
		for k, v := range baseGlobals {
			symbolicValue, err := v.ToSymbolicValue(ctx, encountered)
			if err != nil {
				return nil, fmt.Errorf("failed to convert base global '%s' to symbolic: %w", k, err)
			}
			symbolicBaseGlobals[k] = symbolicValue
		}
	}

	state := core.NewGlobalState(ctx, constants)
	state.Out = conf.Out
	state.Logger = logger
	state.GetBaseGlobalsForImportedModule = func(ctx *core.Context, manifest *core.Manifest) (core.GlobalVariables, error) {
		importedModuleGlobals := maps.Clone(baseGlobals)
		env, err := env_ns.NewEnvNamespace(ctx, nil, conf.AllowMissingEnvVars)
		if err != nil {
			return core.GlobalVariables{}, err
		}

		importedModuleGlobals["env"] = env
		baseGlobalKeys := maps.Keys(importedModuleGlobals)
		return core.GlobalVariablesFromMap(importedModuleGlobals, baseGlobalKeys), nil
	}
	state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
		return maps.Clone(core.DEFAULT_NAMED_PATTERNS), maps.Clone(core.DEFAULT_PATTERN_NAMESPACES)
	}
	state.SymbolicBaseGlobalsForImportedModule = symbolicBaseGlobals
	state.OutputFieldsInitialized.Store(true)

	return state, nil
}

// NewDefaultState creates a new Context with the default patterns.
func NewDefaultContext(config default_state.DefaultContextConfig) (*core.Context, error) {

	ctxConfig := core.ContextConfig{
		Permissions:          config.Permissions,
		ForbiddenPermissions: config.ForbiddenPermissions,
		Limits:               config.Limits,
		HostResolutions:      config.HostResolutions,
		ParentContext:        config.ParentContext,
		ParentStdLibContext:  config.ParentStdLibContext,
		Filesystem:           config.Filesystem,
		OwnedDatabases:       config.OwnedDatabases,
	}

	if ctxConfig.Filesystem == nil && ctxConfig.ParentContext == nil {
		ctxConfig.Filesystem = fs_ns.GetOsFilesystem()
	}

	if ctxConfig.ParentContext != nil {
		if err, _ := ctxConfig.Check(); err != nil {
			return nil, err
		}
	}

	ctx := core.NewContext(ctxConfig)

	for k, v := range core.DEFAULT_NAMED_PATTERNS {
		ctx.AddNamedPattern(k, v)
	}

	for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
		ctx.AddPatternNamespace(k, v)
	}

	return ctx, nil
}
