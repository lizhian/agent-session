# Agent Session

Agent Session is a Go CLI for finding, filtering, configuring, and reopening local sessions created by Claude Code, Codex, OpenCode, and Pi Coding Agent.

## Language

**Agent provider**:
A supported coding-agent product whose local sessions can be browsed and reopened by Agent Session.
_Avoid_: Agent, backend

**Dispatcher binary**:
The single `agent-session` executable that selects an **Agent provider** from either its invoked command name or its first subcommand.
_Avoid_: Provider CLI, provider script

**Public command**:
The user-facing command name that launches one **Agent provider** through the **Dispatcher binary**.
_Avoid_: Alias, internal command

**Provider implementation**:
The provider-specific code that discovers sessions, loads transcripts, builds launch commands, and exposes configuration actions for one **Agent provider**.
_Avoid_: Provider CLI, agent file

**Provider interface**:
The shared contract implemented by every **Provider implementation**.
_Avoid_: Abstract provider, generic backend

**Common support module**:
Shared code used by multiple **Provider implementations** for picker state, config handling, rendering, JSONL parsing, permissions, or command launching.
_Avoid_: Provider code

**Session**:
A local conversation record created by an **Agent provider** for a project directory.
_Avoid_: Chat, transcript file

**Session directory**:
A provider-native storage directory where one **Agent provider** reads and writes **Sessions**.
_Avoid_: Config directory, home directory

**Workspace**:
A project directory that has one or more **Sessions** for an **Agent provider**.
_Avoid_: Folder list, project row

**Session picker**:
The interactive TUI used to search **Sessions**, switch **Workspaces**, open configuration actions, preview transcripts, and choose what to launch.
_Avoid_: Menu, prompt

**Picker view**:
A navigable screen inside the **Session picker**.
_Avoid_: Page, mode

**Configurations view**:
A **Picker view** for provider-specific actions that change the selected **Agent provider** configuration.
_Avoid_: Settings screen, right-click menu

**Manager hub**:
A global **Picker view** reached from the session list that groups cross-provider management entries such as skills management.
_Avoid_: Settings screen, provider config page

**Manager entry**:
A selectable row inside the **Manager hub** that opens one cross-provider management workflow.
_Avoid_: Configuration action, provider setting

**Global skills view**:
A **Picker view** for selecting which **Installed skills** are enabled in the global user scope.
_Avoid_: Source detail, provider config

**Skill installation repository**:
A local storage directory owned by Agent Session where installed skills are kept independently from whether they are enabled globally or for one project.
_Avoid_: Global skills dir, provider dir

**Skill source**:
An installable upstream source that can yield one or more installed skills.
_Avoid_: Installed skill, activation target

**Skill source directory name**:
The filesystem-safe directory name derived from one **Skill source** for storage inside the **Skill installation repository**.
_Avoid_: Installed skill name, local alias

**Installed skill**:
A single skill materialized inside the **Skill installation repository** and identified independently from the **Skill source** that produced it.
_Avoid_: Source repo, link target

**Installed skill description**:
A short summary string associated with one **Installed skill** and used for picker display.
_Avoid_: Source description, skill body

**Installed skill catalog**:
A local Agent Session record that tracks installed skills and their activation state across scopes.
_Avoid_: Source manifest, provider config

**Skill activation target**:
A directory that receives symlinks to installed skills so they become active in one scope, such as the global user scope or the current project scope.
_Avoid_: Install dir, copy target

**Configuration action**:
A selectable operation in the **Configurations view**, such as choosing a **Model provider** or editing model selections.
_Avoid_: Setting row

**Configuration item**:
A selectable choice presented by a **Configuration action**.
_Avoid_: Option row

**Multi-select configuration list**:
A **Picker view** where multiple **Configuration items** can be toggled before saving.
_Avoid_: Session list

**Model provider**:
A selectable model backend configuration inside an **Agent provider**.
_Avoid_: Provider

**Model provider selection state**:
Provider config state that records the current **Model provider** when the **Agent provider** supports that concept.
_Avoid_: Default provider inference

**Permission mode**:
A launch-safety level selected before starting or resuming a **Session**.
_Avoid_: Install mode, trust mode

**Permission mode selection state**:
Provider config state that records the last selected **Permission mode** for an **Agent provider**.
_Avoid_: Install config

**Provider-managed environment**:
Environment fields inside a Claude Code **Agent provider** config that belong to the selected **Model provider**.
_Avoid_: All env fields

**Conversation transcript**:
Provider-loaded chronological user and assistant dialogue for one **Session**, excluding tool noise and internal events.
_Avoid_: Session summary, user-only transcript

**Session preview**:
A temporary **Picker view** for reading the selected **Session**'s **Conversation transcript** before reopening it.
_Avoid_: Editor-like transcript browser

**Conversation message preview**:
A length-limited rendering of one **Conversation transcript** message body.
_Avoid_: Full message dump

**Trust state**:
Provider-native project configuration that marks a project directory as trusted before launching an agent.
_Avoid_: Permission mode

## Relationships

- A **Public command** names exactly one **Agent provider**: `c` names Claude Code, `cx` names Codex, `oc` names OpenCode, and `p` names Pi Coding Agent.
- The **Dispatcher binary** may be invoked as `c`, `cx`, `oc`, or `p`, or as `agent-session c`, `agent-session cx`, `agent-session oc`, or `agent-session p`.
- `cc` remains a legacy compatibility entrypoint for Claude Code, but it is not the canonical **Public command**.
- A **Provider implementation** belongs to exactly one **Agent provider**.
- Every **Provider implementation** implements the **Provider interface**.
- **Provider implementations** own provider-specific session discovery because Claude Code, Codex, OpenCode, and Pi Coding Agent store **Sessions** differently.
- **Common support modules** must not own provider-specific storage rules.
- A **Workspace** contains zero or more currently matching **Sessions** for one **Agent provider**.
- Pi Coding Agent has a **Session directory** that can be set independently from its configuration directory.
- Pi Coding Agent launches use the selected **Session directory** so reopening a **Session** resolves against the same storage used by discovery.
- Pi Coding Agent launches use the selected configuration directory through `PI_CODING_AGENT_DIR`.
- A **Session picker** starts in the session list and can navigate to **Workspaces**, the **Configurations view**, and **Session preview**.
- Pressing Left from the session list opens the **Manager hub**.
- A **Session preview** loads a **Conversation transcript** lazily for the selected **Session**.
- A **Conversation transcript** includes useful user and assistant messages, not tool calls or internal records.
- A **Conversation message preview** limits one message body to 500 non-whitespace Unicode runes by keeping the first 250 and last 250 non-whitespace runes while preserving original whitespace.
- A **Configurations view** belongs to the current **Agent provider**.
- A **Manager hub** is shared by all **Public commands** and is not owned by any one **Agent provider**.
- A **Manager entry** opens a cross-provider workflow from the **Manager hub**.
- A **Global skills view** manages activation of **Installed skills** in the global user scope.
- A **Skill source** may produce one or more **Installed skills**.
- A **Skill source directory name** is derived from a **Skill source** for local storage.
- An **Installed skill** is stored independently from the **Skill source** that produced it.
- An **Installed skill** may have one **Installed skill description**.
- A **Skill installation repository** stores installed skills once for Agent Session.
- A **Skill installation repository** stores **Installed skills**, not raw **Skill sources**.
- A **Skill installation repository** may group **Installed skills** under one **Skill source directory name** per **Skill source**.
- An **Installed skill catalog** records Agent Session-owned metadata for installed skills, including each **Installed skill description** when available.
- Updating a **Skill source** mirrors the source's current **Installed skills** into the **Skill installation repository** and the **Installed skill catalog**, removing previously installed skills that no longer exist in the source.
- A **Skill activation target** enables a skill by symlinking from the **Skill installation repository** into a scope-specific directory.
- A **Configuration action** may show direct **Configuration items** or a **Multi-select configuration list**.
- Claude Code and Codex expose **Model provider** selection as a **Configuration action**.
- OpenCode exposes `@ai-sdk/*` provider model lists, default model, and small model as **Configuration actions**.
- Pi Coding Agent exposes no **Configuration actions** in Agent Session until its configuration workflow is intentionally added.
- **Model provider selection state** is authoritative before fallback native model-provider fields when an **Agent provider** supports both.
- Codex **Model provider selection state** uses top-level `model_provider_selected`, falling back to native `model_provider` only when no selected provider is recorded.
- Claude Code **Model provider selection state** uses `model_provider_selected`; the selected provider's fields are copied into `env`.
- Claude Code **Model provider** switching backs up current **Provider-managed environment** fields into the previously selected provider before applying the new provider.
- Claude Code **Model provider** switching updates only the **Provider-managed environment** and preserves unrelated global `env` fields.
- Claude Code model setting changes require a valid **Model provider selection state**.
- Claude Code **Model provider** model choices are discovered from the selected provider's `GET /v1/models` endpoint.
- Claude Code model discovery uses the selected **Model provider**'s own `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN`, not fallback `env` values.
- Claude Code Haiku, Opus, and Sonnet model settings use the same discovered model list without automatic name-based filtering.
- Codex **Model provider** switching only manages `auth.json` for providers that explicitly declare `auth_json`: the previous provider is backed up only if it already has `auth_json`, and the target provider writes `auth.json` only if it has `auth_json`.
- Codex **Model provider** switching may attempt thread sync through `codex-threadripper`; missing `codex-threadripper` means sync is skipped, not failed.
- OpenCode can read JSONC-style config but writes standard formatted JSON.
- OpenCode **Permission mode selection state** uses native `permission`, and legacy `permission_mode_selected` is migrated away.
- Claude Code and Codex support default, auto, and full **Permission modes**.
- OpenCode supports default and full **Permission modes** only.
- Pi Coding Agent supports only the default **Permission mode**; its tool-selection flags are not **Permission modes**.
- Claude Code and Codex update **Trust state** before launching; OpenCode has no Agent Session-managed **Trust state**.

## Example dialogue

> **Dev:** "If the user runs `agent-session cx`, is `cx` a separate binary?"
> **Domain expert:** "No. `cx` is a **Public command** handled by the same **Dispatcher binary**. It selects the Codex **Agent provider**."

> **Dev:** "Should a provider package parse another provider's session files?"
> **Domain expert:** "No. A **Provider implementation** owns only its own **Agent provider** storage rules and shares only generic behavior through **Common support modules**."

> **Dev:** "When the user presses Right from the session list, do we open model settings immediately?"
> **Domain expert:** "No. The **Session picker** first opens **Workspaces**; pressing Right again opens the **Configurations view** for the current **Agent provider**."

> **Dev:** "Does pressing Space load every transcript upfront?"
> **Domain expert:** "No. Space opens a **Session preview** and lazily loads the selected **Session**'s **Conversation transcript**."

> **Dev:** "Should the **Conversation transcript** include tool output?"
> **Domain expert:** "No. It includes useful user and assistant dialogue, excluding tool noise and internal events."

> **Dev:** "When switching Claude Code **Model providers**, can we replace the whole `env` map?"
> **Domain expert:** "No. Only the **Provider-managed environment** is copied from the selected **Model provider**; unrelated global `env` fields stay unchanged."

> **Dev:** "If Claude Code has no selected **Model provider**, should model setting changes guess one?"
> **Domain expert:** "No. First select a **Model provider**; model setting changes must update `env` and the selected provider together."

> **Dev:** "If a third-party Claude model ID does not contain Haiku, Opus, or Sonnet, should the picker hide it?"
> **Domain expert:** "No. Claude Code Haiku, Opus, and Sonnet actions share the discovered model list because third-party **Model providers** often use aliases."

> **Dev:** "When switching Codex **Model providers**, should every provider update `auth.json`?"
> **Domain expert:** "No. Only providers that explicitly declare `auth_json` participate in Agent Session-managed `auth.json` backup and restore."

> **Dev:** "If `codex-threadripper` is not installed, did Codex provider switching fail?"
> **Domain expert:** "No. Missing `codex-threadripper` means thread sync is skipped after the **Model provider** switch."

> **Dev:** "Should OpenCode store Agent Session's last permission choice in `permission_mode_selected`?"
> **Domain expert:** "No. OpenCode **Permission mode selection state** uses native `permission`; legacy `permission_mode_selected` is migrated away."

> **Dev:** "Can Pi Coding Agent's `--tools` flags be treated as Agent Session **Permission modes**?"
> **Domain expert:** "No. Pi Coding Agent currently has only the default **Permission mode** in Agent Session; tool allowlists are a different concept."

> **Dev:** "Should Pi Coding Agent's native `pi config` workflow appear in Agent Session's **Configurations view**?"
> **Domain expert:** "No. Pi Coding Agent has no Agent Session **Configuration actions** until that workflow is designed separately."

## Flagged ambiguities

- "provider" is resolved as **Model provider** when it refers to a selectable backend inside an **Agent provider**.
- "agent" is resolved as **Agent provider** when it refers to Claude Code, Codex, OpenCode, or Pi Coding Agent.
- "CLI" is resolved as **Dispatcher binary** when discussing `agent-session` itself, and **Public command** when discussing `c`, `cx`, `oc`, or `p`.
- "Provider CLI" is a retired term from the old multi-script layout; use **Provider implementation** for provider-specific Go packages.
- "alias" is resolved as **Public command** unless the discussion is specifically about shell function installation.
- "settings" is resolved as **Configurations view** when discussing the picker screen.
- "Session transcript" is resolved as **Conversation transcript** when discussing preview contents.
- "preview messages" is resolved as **Session preview** backed by a lazily loaded **Conversation transcript**, not the already loaded session summary.
- "trust" is resolved as **Trust state** when marking a project directory trusted, and as **Permission mode** only when discussing launch-safety levels.
