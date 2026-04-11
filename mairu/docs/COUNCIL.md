# Council Mode

Council Mode is a multi-agent architectural orchestration layer designed for complex development tasks. When enabled, Mairu tasks are no longer executed by a single agent but are instead managed by a specialized council of four autonomous roles, ensuring higher code quality, security, and project alignment.

## The Four Roles
Each role has a distinct functional impact on the task workflow:

- **Architect:** Defines the system structure and ensures changes adhere to the project's high-level design.
- **App Developer:** Executes the actual coding tasks and implements logic according to the Architect's plan.
- **Security:** Reviews all changes for vulnerabilities and ensures compliance with security standards.
- **Tests Evangelist:** Enforces strict adherence to testing standards (e.g., unit/integration tests). **Note:** This role will halt execution if required tests are missing or failing.

## Runtime Controls

Council Mode can be managed via the CLI or the interactive TUI.

### CLI
Start Council Mode by using the `--council` flag with the `minion` command:
```bash
mairu minion --council
```

### TUI
Once in the TUI, you can toggle or inspect Council Mode using runtime commands:
- `/council on`: Enables Council Mode.
- `/council off`: Disables Council Mode.
- `/council status`: Provides real-time introspection of the current agent orchestration state.

## Performance Caveats
- **Resource Usage:** Council Mode requires significantly higher token consumption and introduces more latency due to inter-agent communication and multi-step reasoning.
- **When to use:** Use Council Mode for complex refactors, multi-module dependency management, or critical system changes. It is not recommended for trivial tasks or in latency-sensitive CI/CD pipelines.

## Integration & Verification
The Test Evangelist role verifies that your work aligns with the standards defined in the project's `CONTRIBUTING.md` and `Makefile`. If you initiate Council Mode, ensure your project structure facilitates automated testing; the agent will perform a mandatory validation check and will refuse to complete the task if testing requirements are unmet.
