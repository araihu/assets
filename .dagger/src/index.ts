import {
  argument,
  dag,
  Container,
  Directory,
  File,
  func,
  object,
  Secret,
} from "@dagger.io/dagger"

const GO_IMAGE =
  "golang:1.26.5-bookworm@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd"

const SOURCE_EXCLUDES = [
  ".git",
  ".git/**",
  ".dagger/node_modules",
  ".dagger/node_modules/**",
  ".cache",
  ".cache/**",
]

type Input = Record<string, string>
type CacheNamespace = "trusted" | "pr"

@object()
export class Assets {
  /** Synchronize and verify every locked acquisition input. */
  @func({ cache: "never" })
  async acquisition(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    payload: File,
  ): Promise<string> {
    const input = await this.input(payload, ["refresh_nonce"])
    this.nonce(input.refresh_nonce)
    return this.base(source, "trusted")
      .withEnvVariable("NETWORK_NONCE", input.refresh_nonce)
      .withExec(["bash", "scripts/dagger/acquisition.sh"])
      .stdout()
  }

  /** Run complete offline source, generated-artifact, and release validation. */
  @func()
  async ci(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    payload: File,
  ): Promise<string> {
    const input = await this.input(payload, ["cache_namespace", "run_nonce"])
    const cacheNamespace = this.cacheNamespace(input.cache_namespace)
    this.nonce(input.run_nonce)
    return this.rsvg(this.base(source, cacheNamespace), cacheNamespace)
      .withExec(["bash", "scripts/dagger/ci.sh"])
      .stdout()
  }

  /** Build a campaign candidate and compare its digest with durable accepted state. */
  @func({ cache: "never" })
  async campaignPlan(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    payload: File,
    githubToken: Secret,
    ahairuToken: Secret,
  ): Promise<Directory> {
    const input = await this.input(payload, [
      "repository", "ahairu_owner", "ahairu_repository", "date",
      "github_server_url", "github_api_url", "refresh_nonce",
    ])
    this.repository(input.repository)
    this.slug(input.ahairu_owner, "ahairu_owner")
    this.slug(input.ahairu_repository, "ahairu_repository")
    this.url(input.github_server_url, "github_server_url")
    this.url(input.github_api_url, "github_api_url")
    this.nonce(input.refresh_nonce)
    if (input.date !== "" && !/^\d{4}-\d{2}-\d{2}$/.test(input.date)) throw new Error("date must use YYYY-MM-DD")
    return this.base(source, "trusted")
      .withSecretVariable("GH_TOKEN", githubToken)
      .withSecretVariable("AHAIRU_TOKEN", ahairuToken)
      .withEnvVariable("GITHUB_REPOSITORY", input.repository)
      .withEnvVariable("AHAIRU_OWNER", input.ahairu_owner)
      .withEnvVariable("AHAIRU_REPOSITORY", input.ahairu_repository)
      .withEnvVariable("REQUESTED_DATE", input.date)
      .withEnvVariable("GITHUB_SERVER_URL", input.github_server_url)
      .withEnvVariable("GITHUB_API_URL", input.github_api_url)
      .withEnvVariable("NETWORK_NONCE", input.refresh_nonce)
      .withExec(["bash", "scripts/dagger/campaign-plan.sh"])
      .directory("/out")
  }

  /** Dispatch one changed campaign artifact to Ahairu. */
  @func({ cache: "never" })
  async campaignDispatch(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    plan: Directory,
    payload: File,
    ahairuToken: Secret,
  ): Promise<string> {
    const input = await this.input(payload, [
      "repository", "ahairu_owner", "ahairu_repository", "github_server_url",
      "github_api_url", "artifact_id", "artifact_url", "artifact_sha256",
      "source_revision", "effect_nonce",
    ])
    this.repository(input.repository)
    this.slug(input.ahairu_owner, "ahairu_owner")
    this.slug(input.ahairu_repository, "ahairu_repository")
    this.url(input.github_server_url, "github_server_url")
    this.url(input.github_api_url, "github_api_url")
    this.nonce(input.effect_nonce)
    return this.base(source, "trusted")
      .withDirectory("/plan", plan)
      .withSecretVariable("GH_TOKEN", ahairuToken)
      .withEnvVariable("CHANNEL_ARTIFACT_ID", input.artifact_id)
      .withEnvVariable("CHANNEL_ARTIFACT_URL", input.artifact_url)
      .withEnvVariable("CHANNEL_ARTIFACT_SHA256", input.artifact_sha256)
      .withEnvVariable("SOURCE_REVISION", input.source_revision)
      .withEnvVariable("EFFECT_NONCE", input.effect_nonce)
      .withEnvVariable("GITHUB_REPOSITORY", input.repository)
      .withEnvVariable("AHAIRU_OWNER", input.ahairu_owner)
      .withEnvVariable("AHAIRU_REPOSITORY", input.ahairu_repository)
      .withEnvVariable("GITHUB_SERVER_URL", input.github_server_url)
      .withEnvVariable("GITHUB_API_URL", input.github_api_url)
      .withEnvVariable("NETWORK_NONCE", input.effect_nonce)
      .withExec(["bash", "scripts/dagger/campaign-dispatch.sh"])
      .stdout()
  }

  /** Build and verify immutable release and latest-candidate bundles. */
  @func({ cache: "never" })
  async releaseBundle(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    payload: File,
    githubToken: Secret,
    fanoutAppID: Secret,
    fanoutAppPrivateKey: Secret,
  ): Promise<Directory> {
    const input = await this.input(payload, ["tag", "source_revision", "repository", "refresh_nonce"])
    this.repository(input.repository)
    this.nonce(input.refresh_nonce)
    return this.rsvg(this.base(source, "trusted"), "trusted")
      .withSecretVariable("GH_TOKEN", githubToken)
      .withSecretVariable("ARAIHU_ASSETS_APP_ID", fanoutAppID)
      .withSecretVariable("ARAIHU_ASSETS_APP_PRIVATE_KEY", fanoutAppPrivateKey)
      .withEnvVariable("RELEASE_TAG", input.tag)
      .withEnvVariable("SOURCE_REVISION", input.source_revision)
      .withEnvVariable("GITHUB_REPOSITORY", input.repository)
      .withEnvVariable("NETWORK_NONCE", input.refresh_nonce)
      .withExec(["bash", "scripts/dagger/release-build.sh"])
      .directory("/out")
  }

  /** Create or append missing assets to the matching GitHub Release. */
  @func({ cache: "never" })
  async releasePublish(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    releaseOutput: Directory,
    payload: File,
    githubToken: Secret,
  ): Promise<string> {
    const input = await this.input(payload, ["tag", "repository", "effect_nonce"])
    this.repository(input.repository)
    this.nonce(input.effect_nonce)
    return this.base(source, "trusted")
      .withDirectory("/release-output", releaseOutput)
      .withSecretVariable("GH_TOKEN", githubToken)
      .withEnvVariable("RELEASE_TAG", input.tag)
      .withEnvVariable("EFFECT_NONCE", input.effect_nonce)
      .withEnvVariable("GITHUB_REPOSITORY", input.repository)
      .withEnvVariable("NETWORK_NONCE", input.effect_nonce)
      .withExec(["bash", "scripts/dagger/release-publish.sh"])
      .stdout()
  }

  /** Verify published release bytes and produce immutable fan-out metadata. */
  @func({ cache: "never" })
  async releaseFanoutPlan(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    payload: File,
    githubToken: Secret,
  ): Promise<Directory> {
    const input = await this.input(payload, ["release", "repository", "refresh_nonce"])
    this.repository(input.repository)
    this.nonce(input.refresh_nonce)
    return this.base(source, "trusted")
      .withSecretVariable("RELEASE_GITHUB_TOKEN", githubToken)
      .withEnvVariable("RELEASE", input.release)
      .withEnvVariable("NETWORK_NONCE", input.refresh_nonce)
      .withEnvVariable("GITHUB_REPOSITORY", input.repository)
      .withExec(["bash", "scripts/dagger/release-fanout-plan.sh"])
      .directory("/out")
  }

  /** Dispatch already-verified metadata to every enrolled fallback consumer. */
  @func({ cache: "never" })
  async releaseFanout(
    @argument({ defaultPath: ".", ignore: SOURCE_EXCLUDES }) source: Directory,
    plan: Directory,
    payload: File,
    consumerToken: Secret,
  ): Promise<string> {
    const input = await this.input(payload, ["effect_nonce"])
    this.nonce(input.effect_nonce)
    return this.base(source, "trusted")
      .withDirectory("/fanout", plan)
      .withSecretVariable("GH_TOKEN", consumerToken)
      .withEnvVariable("EFFECT_NONCE", input.effect_nonce)
      .withEnvVariable("NETWORK_NONCE", input.effect_nonce)
      .withExec(["bash", "scripts/dagger/release-fanout.sh"])
      .stdout()
  }

  private async input(file: File, keys: string[]): Promise<Input> {
    const value: unknown = JSON.parse(await file.contents())
    if (value === null || Array.isArray(value) || typeof value !== "object") throw new Error("payload must be a JSON object")
    const record = value as Record<string, unknown>
    if (Object.keys(record).sort().join("\0") !== [...keys].sort().join("\0")) throw new Error("payload keys differ from the exact schema")
    for (const key of keys) if (typeof record[key] !== "string") throw new Error(`payload field ${key} must be a string`)
    return record as Input
  }

  private cacheNamespace(value: string): CacheNamespace {
    if (value !== "trusted" && value !== "pr") throw new Error("unknown cache namespace")
    return value
  }

  private nonce(value: string): void {
    if (!/^[1-9][0-9]*-[1-9][0-9]*$/.test(value) && value !== "local") throw new Error("invalid execution nonce")
  }

  private repository(value: string): void {
    if (value !== "araihu/assets") throw new Error("unexpected source repository")
  }

  private slug(value: string, name: string): void {
    if (!/^[A-Za-z0-9](?:[A-Za-z0-9_.-]{0,98}[A-Za-z0-9])?$/.test(value)) throw new Error(`invalid ${name}`)
  }

  private url(value: string, name: string): void {
    const parsed = new URL(value)
    if (parsed.protocol !== "https:" || parsed.username || parsed.password || parsed.hash) throw new Error(`invalid ${name}`)
  }

  private base(source: Directory, cacheNamespace: CacheNamespace): Container {
    return dag
      .container()
      .from(GO_IMAGE)
      .withExec([
        "bash", "-euo", "pipefail", "-c",
        "apt-get update && apt-get install --yes --no-install-recommends ca-certificates curl git gh make nodejs npm python3 ruby unzip xz-utils && rm -rf /var/lib/apt/lists/*",
      ])
      .withDirectory("/src", source)
      .withDirectory("/baseline", source)
      .withWorkdir("/src")
      .withEnvVariable("GOPROXY", "off")
      .withEnvVariable("GOSUMDB", "off")
      .withEnvVariable("GONOSUMDB", "*")
      .withEnvVariable("MUAMBA_CACHE_DIR", "/root/.cache/muamba")
      .withMountedCache("/root/.cache/go-build", dag.cacheVolume(`araihu-ci-v1-assets-${cacheNamespace}-go-build-1.26.5`))
      .withMountedCache("/go/pkg/mod", dag.cacheVolume(`araihu-ci-v1-assets-${cacheNamespace}-go-mod-1.26.5`))
      .withMountedCache("/root/.cache/muamba", dag.cacheVolume(`araihu-ci-v1-assets-${cacheNamespace}-muamba-v0.0.4`))
  }

  private rsvg(container: Container, cacheNamespace: CacheNamespace): Container {
    return container
      .withMountedCache(
        "/root/.cargo",
        dag.cacheVolume(`araihu-ci-v1-assets-${cacheNamespace}-cargo-rust-1.92.0`),
      )
      .withExec(["bash", "scripts/dagger/install-rsvg.sh"])
      .withEnvVariable("PATH", "/opt/librsvg/bin:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin")
  }
}
