// deno-fmt-ignore-file
// biome-ignore format: generated types do not need formatting
// prettier-ignore
import type { PathsForPages } from 'waku/router'

// prettier-ignore
type Page =
  | { path: '/001-onboarding'; render: 'static' }
  | { path: '/013-getting-started'; render: 'static' }
  | { path: '/035-configuration'; render: 'static' }
  | { path: '/036-runtime'; render: 'static' }
  | { path: '/044-batching'; render: 'static' }
  | { path: '/behaviour/083-equivalence'; render: 'static' }
  | { path: '/behaviour/084-order'; render: 'static' }
  | { path: '/caching/045-caching-effects'; render: 'static' }
  | { path: '/caching/046-cache'; render: 'static' }
  | { path: '/code-style/064-guidelines'; render: 'static' }
  | { path: '/code-style/065-dual'; render: 'static' }
  | { path: '/code-style/066-branded-types'; render: 'static' }
  | { path: '/code-style/067-pattern-matching'; render: 'static' }
  | { path: '/code-style/068-do'; render: 'static' }
  | { path: '/code-style/069-control-flow'; render: 'static' }
  | { path: '/concurrency/007-basic-concurrency'; render: 'static' }
  | { path: '/concurrency/047-fibers'; render: 'static' }
  | { path: '/concurrency/048-deferred'; render: 'static' }
  | { path: '/concurrency/049-queue'; render: 'static' }
  | { path: '/concurrency/050-pubsub'; render: 'static' }
  | { path: '/concurrency/051-semaphore'; render: 'static' }
  | { path: '/concurrency/052-latch'; render: 'static' }
  | { path: '/data-types/070-bigdecimal'; render: 'static' }
  | { path: '/data-types/071-cause'; render: 'static' }
  | { path: '/data-types/072-chunk'; render: 'static' }
  | { path: '/data-types/073-data'; render: 'static' }
  | { path: '/data-types/074-datetime'; render: 'static' }
  | { path: '/data-types/075-duration'; render: 'static' }
  | { path: '/data-types/076-result'; render: 'static' }
  | { path: '/data-types/077-exit'; render: 'static' }
  | { path: '/data-types/078-hash-set'; render: 'static' }
  | { path: '/data-types/079-option'; render: 'static' }
  | { path: '/data-types/080-redacted'; render: 'static' }
  | { path: '/error-management/006-two-error-types'; render: 'static' }
  | { path: '/error-management/014-expected-errors'; render: 'static' }
  | { path: '/error-management/015-unexpected-errors'; render: 'static' }
  | { path: '/error-management/016-fallback'; render: 'static' }
  | { path: '/error-management/017-matching'; render: 'static' }
  | { path: '/error-management/018-retrying'; render: 'static' }
  | { path: '/error-management/019-timing-out'; render: 'static' }
  | { path: '/error-management/020-sandboxing'; render: 'static' }
  | { path: '/error-management/021-error-accumulation'; render: 'static' }
  | { path: '/error-management/022-error-channel-operations'; render: 'static' }
  | { path: '/error-management/023-parallel-and-sequential-errors'; render: 'static' }
  | { path: '/error-management/024-yieldable-errors'; render: 'static' }
  | { path: '/getting-started/002-why-effect'; render: 'static' }
  | { path: '/getting-started/003-the-effect-type'; render: 'static' }
  | { path: '/getting-started/004-installation'; render: 'static' }
  | { path: '/getting-started/005-creating-effects'; render: 'static' }
  | { path: '/getting-started/008-importing-effect'; render: 'static' }
  | { path: '/getting-started/009-running-effects'; render: 'static' }
  | { path: '/getting-started/010-using-generators'; render: 'static' }
  | { path: '/getting-started/011-building-pipelines'; render: 'static' }
  | { path: '/getting-started/012-devtools'; render: 'static' }
  | { path: '/'; render: 'static' }
  | { path: '/observability/031-logging'; render: 'static' }
  | { path: '/observability/032-metrics'; render: 'static' }
  | { path: '/observability/033-tracing'; render: 'static' }
  | { path: '/observability/034-tracking-fibers'; render: 'static' }
  | { path: '/platform/105-introduction'; render: 'static' }
  | { path: '/platform/106-file-system'; render: 'static' }
  | { path: '/platform/107-path'; render: 'static' }
  | { path: '/platform/108-platformlogger'; render: 'static' }
  | { path: '/platform/109-runtime'; render: 'static' }
  | { path: '/platform/110-terminal'; render: 'static' }
  | { path: '/requirements-management/025-services'; render: 'static' }
  | { path: '/requirements-management/026-default-services'; render: 'static' }
  | { path: '/requirements-management/027-layers'; render: 'static' }
  | { path: '/requirements-management/028-layer-memoization'; render: 'static' }
  | { path: '/resource-management/029-introduction'; render: 'static' }
  | { path: '/resource-management/030-scope'; render: 'static' }
  | { path: '/scheduling/037-using-schedules'; render: 'static' }
  | { path: '/scheduling/038-choosing-and-combining-schedules'; render: 'static' }
  | { path: '/scheduling/039-cron'; render: 'static' }
  | { path: '/scheduling/040-cookbook'; render: 'static' }
  | { path: '/schema/085-introduction'; render: 'static' }
  | { path: '/schema/086-getting-started'; render: 'static' }
  | { path: '/schema/087-basic-usage'; render: 'static' }
  | { path: '/schema/088-filters'; render: 'static' }
  | { path: '/schema/089-advanced-usage'; render: 'static' }
  | { path: '/schema/090-projections'; render: 'static' }
  | { path: '/schema/091-transformations'; render: 'static' }
  | { path: '/schema/092-annotations'; render: 'static' }
  | { path: '/schema/093-canonical-codecs'; render: 'static' }
  | { path: '/schema/094-error-messages'; render: 'static' }
  | { path: '/schema/095-error-formatters'; render: 'static' }
  | { path: '/schema/096-classes'; render: 'static' }
  | { path: '/schema/097-default-constructors'; render: 'static' }
  | { path: '/schema/098-effect-data-types'; render: 'static' }
  | { path: '/schema/099-standard-schema'; render: 'static' }
  | { path: '/schema/100-arbitrary'; render: 'static' }
  | { path: '/schema/101-json-schema'; render: 'static' }
  | { path: '/schema/102-equivalence'; render: 'static' }
  | { path: '/schema/103-formatter'; render: 'static' }
  | { path: '/schema/104-compilers'; render: 'static' }
  | { path: '/setup/existing-project'; render: 'static' }
  | { path: '/setup/neovim'; render: 'static' }
  | { path: '/setup/new-project'; render: 'static' }
  | { path: '/setup/other-editors'; render: 'static' }
  | { path: '/setup/react'; render: 'static' }
  | { path: '/setup/svelte'; render: 'static' }
  | { path: '/setup/vibelang-cli'; render: 'static' }
  | { path: '/setup/vite'; render: 'static' }
  | { path: '/setup/vscode'; render: 'static' }
  | { path: '/setup/vue'; render: 'static' }
  | { path: '/setup/zed'; render: 'static' }
  | { path: '/setup'; render: 'static' }
  | { path: '/sink/059-introduction'; render: 'static' }
  | { path: '/sink/060-creating'; render: 'static' }
  | { path: '/sink/061-operations'; render: 'static' }
  | { path: '/sink/062-leftovers'; render: 'static' }
  | { path: '/state-management/041-ref'; render: 'static' }
  | { path: '/state-management/042-synchronizedref'; render: 'static' }
  | { path: '/state-management/043-subscriptionref'; render: 'static' }
  | { path: '/stream/053-introduction'; render: 'static' }
  | { path: '/stream/054-creating'; render: 'static' }
  | { path: '/stream/055-consuming-streams'; render: 'static' }
  | { path: '/stream/056-error-handling'; render: 'static' }
  | { path: '/stream/057-operations'; render: 'static' }
  | { path: '/stream/058-resourceful-streams'; render: 'static' }
  | { path: '/testing/063-testclock'; render: 'static' }
  | { path: '/trait/081-equal'; render: 'static' }
  | { path: '/trait/082-hash'; render: 'static' }
  | { path: '/vibelang-features/111-control-operations-and-handlers'; render: 'static' }
  | { path: '/vibelang-features/112-producers'; render: 'static' }
  | { path: '/vibelang-features/113-building-an-http-api'; render: 'static' }
  | { path: '/vibelang-features/114-vibelang-for-effect-users'; render: 'static' }
  | { path: '/vibelang-features/115-cheat-sheet'; render: 'static' }
  | { path: '/vibelang-features/116-match-and-is'; render: 'static' }

// prettier-ignore
declare module 'waku/router' {
  interface RouteConfig {
    paths: PathsForPages<Page>
  }
  interface CreatePagesConfig {
    pages: Page
  }
}
