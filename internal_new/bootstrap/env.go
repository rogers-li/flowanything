package bootstrap

import (
	"os"
	"strings"
)

const defaultEnvPrefix = "FLOW_ANYTHING"

// RuntimeConfigFromEnv builds RuntimeConfig from environment variables. The
// default prefix is FLOW_ANYTHING, for example FLOW_ANYTHING_BUNDLE_PATH.
func RuntimeConfigFromEnv(prefix string) RuntimeConfig {
	if strings.TrimSpace(prefix) == "" {
		prefix = defaultEnvPrefix
	}
	return RuntimeConfig{
		BundlePath:        env(prefix, "BUNDLE_PATH"),
		DraftBundlePath:   env(prefix, "DRAFT_BUNDLE_PATH"),
		PreviewBundlePath: env(prefix, "PREVIEW_BUNDLE_PATH"),
		ReleaseBundlePath: env(prefix, "RELEASE_BUNDLE_PATH"),
		BundleID:          env(prefix, "BUNDLE_ID"),
		DebugSessionPath:  env(prefix, "DEBUG_SESSION_PATH"),
		RunHistoryPath:    env(prefix, "RUN_HISTORY_PATH"),
		TraceStorePath:    env(prefix, "TRACE_STORE_PATH"),
		Addr:              env(prefix, "RUNTIME_ADDR"),
		ModelProvider:     env(prefix, "MODEL_PROVIDER"),
		ModelBaseURL:      env(prefix, "MODEL_BASE_URL"),
		ModelAPIKey:       env(prefix, "MODEL_API_KEY"),
		MockContent:       env(prefix, "MOCK_CONTENT"),
	}
}

func env(prefix string, name string) string {
	return strings.TrimSpace(os.Getenv(prefix + "_" + name))
}

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}
