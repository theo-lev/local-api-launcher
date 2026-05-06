# Global JDK path setting

## What to build

Add a global JDK path field in the settings UI, alongside the existing Maven path field. When launching a Spring Boot process, the app sets `JAVA_HOME` to this path so that Maven uses the correct JDK — the one that has the corporate CA certificate in its `cacerts` truststore. This resolves PKIX errors when APIs need to reach internal HTTPS servers.

## Acceptance criteria

- [ ] Settings UI has a JDK path input field, persisted in `config.json`
- [ ] When a process is started, `JAVA_HOME` is set to the configured JDK path in the process environment
- [ ] If no JDK path is configured, behaviour is unchanged (uses system `JAVA_HOME` or PATH)
- [ ] Maven path setting continues to work independently

## Blocked by

None - can start immediately
