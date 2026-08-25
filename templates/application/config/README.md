# Runtime configuration

Local development uses the complete Admin package's embedded, redacted
configuration through `CONFIG_PROVIDER=fs`; no configuration file needs to be
copied into this directory. `mss setup` initializes the local SQLite database;
the first interactive run uses its built-in hidden password prompt.
Non-interactive automation provides the same one-use value through
`MSS_ADMIN_INITIAL_PASSWORD` for the setup process only.

Keep deployment-specific configuration and every secret outside source
control. Select and provision a production configuration provider through the
deployment platform instead of committing an `application-*.yml` file here.
