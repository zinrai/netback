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
netback -routerdb routerdb.yaml -model model.yaml -output ./configs
```

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `-routerdb` | (required) | Path to routerdb.yaml |
| `-model` | (required) | Path to model.yaml |
| `-output` | `./configs` | Output directory |
| `-workers` | `5` | Number of concurrent connections |
| `-timeout` | `30s` | Default connection timeout |

## routerdb.yaml

```yaml
devices:
  - name: spine-01
    ip: 192.168.1.1
    model: eos
    group: datacenter-tokyo
    username: admin
    password: secret123

  - name: leaf-01
    ip: 192.168.1.10
    model: eos
    group: datacenter-tokyo
    username: admin
    password: secret123
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| name | Yes | Device identifier (used for output filename) |
| ip | Yes | IP address or hostname |
| model | Yes | Model name (defined in model.yaml) |
| group | Yes | Output subdirectory |
| username | Yes | Authentication username |
| password | Yes | Authentication password |
| port | No | SSH port (default: 22) |
| timeout | No | Connection timeout (default: 30s) |

### Output Structure

Configs are organized by group:

```
./configs/
└── datacenter-tokyo/
    ├── spine-01
    └── leaf-01
```

## model.yaml

```yaml
models:
  eos:
    prompt: '^.+[#>]$'
    comment: '! '

    connection:
      post_login:
        - "enable"
        - "terminal length 0"
      pre_logout: "exit"

    secrets:
      - pattern: '^(snmp-server community).*'
        replace: '\1 <configuration removed>'
      - pattern: '(secret \w+) (\S+).*'
        replace: '\1 <secret hidden>'

    comments:
      - "show inventory | no-more"

    commands:
      - "show running-config | no-more | exclude ! Time:"
```

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

### comments vs commands

- `comments`: All output lines are prefixed with the `comment` string
- `commands`: Only the first line (command echo) and last line (prompt) are commented

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
