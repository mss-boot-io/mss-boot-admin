# Runtime configuration

Keep environment-specific Admin configuration outside source control. Use
`application-local.example.yml` only for redacted local defaults, and inject
production secrets through the deployment platform.
