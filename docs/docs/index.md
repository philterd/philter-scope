# PhilterScope

PhilterScope is a standalone CLI tool for PII redaction auditing and policy optimization. It allows you to audit the performance of [Philter](https://www.github.com/philterd/philter) by comparing its redaction results against a "golden dataset" of labeled PII.

PhilterScope is a core part of our framework for implementing PII/PHI redaction solutions. To learn more, visit us at [Philterd](https://www.philterd.ai). Professional services are available for custom PII redaction solutions.

![PhilterScope dashboard](dashboard.png)

## Documentation

- [Running PhilterScope](running.md): Detailed explanation of commands and flags.
- [Environment Variables](environment-variables.md): Available environment variables for configuration.
- [Development](development.md): Instructions for building and testing the project.

## Quick Start

The following command will compare the raw text in the `examples/raw` directory against the golden dataset in `examples/golden` and generate an HTML report in the `examples/` directory.

```
./philterscope-audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75
```

To store the audit in MongoDB database, provide the database connection information:

```
PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope-audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75
```

The following command will launch the evaluation UI on port 5000 and load the report generated in the previous step.

```
./philterscope-serve --report ./examples/report.json --port 5000
```

Likewise with audits, to view audit results stored in MongoDB database, provide the database connection information:

```
PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope-serve --report ./examples/report.json --port 5000
```

See the [documentation](running.md) for more details and options.

## License

Copyright 2026 Philterd, LLC. "Philter" is a registered trademark of Philterd, LLC. All rights reserved.

Licensed under the Apache License, Version 2.0.