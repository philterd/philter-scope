# PhilterScope

PhilterScope is a standalone CLI tool for PII redaction auditing and policy optimization. It allows you to audit the performance of [Philter](https://www.github.com/philterd/philter) by comparing its redaction results against a "golden dataset" of labeled PII.

PhilterScope is a core part of our framework for implementing PII/PHI redaction solutions. To learn more, visit us at [Philterd](https://www.philterd.ai).

![PhilterScope dashboard](docs/dashboard.png)

## Documentation

- [Running PhilterScope](docs/running.md): Detailed explanation of commands and flags.
- [Environment Variables](docs/environment-variables.md): Available environment variables for configuration.
- [Development](docs/development.md): Instructions for building and testing the project.

## Quick Start

The following command will compare the raw text in the `examples/raw` directory against the golden dataset in `examples/golden` and generate an HTML report in the `examples/` directory.

```
./philterscope audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75
```

Check the `examples/report.html` file for an example of the generated report. See the [documentation](docs/running.md) for more details and options.

## License

Copyright 2026 Philterd, LLC. "Philter" is a registered trademark of Philterd, LLC. All rights reserved.

Licensed under the Apache License, Version 2.0.