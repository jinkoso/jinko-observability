package conventions

// Canonical values for the Datadog "env" tag (OTel deployment.environment).
//
// These four values are the entire vocabulary used across the Jinko estate:
// every service, in every deployment, tags its traces, logs, and metrics with
// exactly one of them. Region, cluster, and cell information belongs in the
// host/cluster tags — never in "env". A value like "prod-us" splits one logical
// environment across two APM buckets and silently breaks every cross-service
// query, dashboard, and monitor that groups by env.
//
// Review decision 2026-07-07, JIN-1570.
const (
	EnvDev     = "dev"
	EnvSandbox = "sandbox"
	EnvPreprod = "preprod"
	EnvProd    = "prod"
)

// Environments lists the canonical env values, least to most production-like.
// Use it to enumerate the vocabulary — error messages, config documentation,
// table-driven tests. Use IsCanonicalEnvironment to check a single value.
//
// Treat it as read-only: IsCanonicalEnvironment does not consult this slice, so
// mutating it changes what callers print without changing what they accept.
var Environments = []string{EnvDev, EnvSandbox, EnvPreprod, EnvProd}

// IsCanonicalEnvironment reports whether env is one of the four canonical
// Datadog env values (JIN-1570).
//
// The match is exact: no case folding and no whitespace trimming. "Prod",
// "PROD", and "prod-us" are all rejected, because Datadog tag values are
// case-sensitive and an almost-right env tag is as damaging as a wrong one.
func IsCanonicalEnvironment(env string) bool {
	switch env {
	case EnvDev, EnvSandbox, EnvPreprod, EnvProd:
		return true
	default:
		return false
	}
}
