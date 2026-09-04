# netback

Declarative network device configuration backup tool.

## Why netback?

Network device configuration backup is essentially about running commands like `show running-config` and saving the output to files.

Existing solutions such as Oxidized integrate web UIs, REST APIs, scheduling, and version control into a monolithic package. These features are valuable, but customizing device interactions requires programming in their implementation language.

I felt this creates two friction points:

- **High ongoing cost**: Requiring programming knowledge for a task that could be described declaratively
- **Skill set mismatch**: The tools demand different expertise than what the task itself requires — CLI familiarity and regex are often sufficient, yet Ruby or Python proficiency becomes necessary

netback focuses solely on the core task: execute commands and save output. Everything else — scheduling, version control, notifications — is delegated to external tools that already do those jobs well.

Configuration is purely declarative: YAML files and regex patterns. What you write is exactly what gets executed.

## Usage

```bash
$ netback -routerdb routerdb.yaml -model model.yaml -output ./configs
```

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `-routerdb` | (required) | Path to routerdb.yaml |
| `-model` | (required) | Path to model.yaml |
| `-output` | `./configs` | Output directory |
| `-workers` | `5` | Number of concurrent connections |
| `-timeout` | `30s` | Default timeout, used for devices that do not set their own |
| `-version` | | Print the version and exit |

A device fails once it has been silent for longer than its timeout, so the
value bounds how long the tool waits for the next piece of output rather than
how long a command may take in total.

### Exit status

netback exits 1 if any device failed and 0 otherwise. Progress and failures are
written to standard error, so a scheduler can treat a non-zero exit as the
signal to surface the log:

```
2026/01/19 20:04:46 spine-01: connecting...
2026/01/19 20:04:47 spine-01: ok
2026/01/19 20:04:47 leaf-01: failed - execute "show running-config": timeout waiting for .+[#>]\s*$
2026/01/19 20:04:47 Completed: 1 success, 1 failed
```

One device failing does not stop the others, and a device that fails leaves its
previous backup untouched.

## Defining Devices

Device connection information is defined in `routerdb.yaml`.

See [examples/routerdb.yaml](./examples/routerdb.yaml) for a working example.

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| name | Yes | Device identifier (used for output filename) |
| ip | Yes | IP address or hostname |
| model | Yes | Model name (defined in model.yaml) |
| group | Yes | Output subdirectory |
| username | Yes | Authentication username |
| password_file | Yes | Path to a file holding the password |
| port | No | SSH port (default: 22) |
| timeout | No | Timeout for connecting and for waiting on output (default: `-timeout`) |

### Passwords

Passwords are always read from a file, so `routerdb.yaml` holds no credentials
and can be version controlled as it is:

```yaml
devices:
  - name: spine-01
    ip: 192.0.2.1
    model: eos
    group: datacenter-tokyo
    username: admin
    password_file: /etc/netback/spine-01.pw
```

The file holds the password and nothing else. One trailing newline is ignored,
so `echo` is enough to create it. Devices that share a credential can point at
the same file.

Nothing about the path is special, which is what makes it composable: a file
placed by systemd `LoadCredential=`, a mounted secret, or a file written by a
secret manager before the run all work the same way.

Every password file is read before the first device is contacted, so an
unreadable one stops the run instead of turning into a failed login part way
through.

### Output Structure

Configs are organized by group:

```
./configs/
└── datacenter-tokyo/
    ├── spine-01
    └── leaf-01
```

## Defining Models

Device interaction patterns are defined in `model.yaml`.

See [examples/model.yaml](./examples/model.yaml) for a working example.

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| prompt | Yes | Regex pattern to detect command prompt |
| comment | No | Prefix for comment lines |
| connection.post_login | No | Commands to run after login |
| connection.pre_logout | No | Command to run before logout |
| secrets | No | Patterns to mask sensitive information |
| comments | No | Commands whose output is entirely commented |
| commands | Yes | Commands to collect configuration |

### prompt

`prompt` is matched against the response as a whole, so `^` and `$` anchor to
the response rather than to a line. Paging is turned off with a
`connection.post_login` command or a per-command modifier such as `| no-more`.

### secrets

`secrets` patterns are matched line by line, so `^` and `$` anchor to a line:

```yaml
    secrets:
      - pattern: '^(snmp-server community).*'
        replace: '$1 <configuration removed>'
```

### comments vs commands

- `comments`: Every non-empty output line is prefixed with the `comment` string
- `commands`: Only the command echo and the prompt that follows the output are commented

This separation allows you to:
- Use `comments` for informational output like `show version`, `show inventory`
- Use `commands` for configuration backup like `show running-config`

### Output Example

```
! show inventory | no-more
! Arista DCS-7050TX-64
! Serial: ABC123
! spine-01#
! show running-config | no-more | exclude ! Time:
hostname spine-01
interface Ethernet1
   description uplink
!
! spine-01#
```

The commented first/last lines in `commands` output indicate which command produced which output, making it easier to debug issues.

## License

This project is licensed under the [MIT License](./LICENSE).
