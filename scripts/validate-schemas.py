#!/usr/bin/env python3
"""Validate published schemas and actual private result/config pairs."""
import argparse
import copy
import json
from pathlib import Path
from jsonschema import Draft202012Validator, FormatChecker, ValidationError


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--evidence', type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    validators = {}
    for path in sorted((root / 'schemas').glob('*.schema.json')):
        schema = json.loads(path.read_text())
        Draft202012Validator.check_schema(schema)
        version = schema['properties']['schemaVersion']['const']
        validators[version] = Draft202012Validator(schema, format_checker=FormatChecker())

    def validate(path):
        sample = json.loads(path.read_text())
        validators[sample['schemaVersion']].validate(sample)
        return sample

    validate(root / 'examples/go-project/itervyn.json')
    validate(root / '.itervyn-policy/itervyn.json')
    records = sorted(args.evidence.rglob('result.json')) if args.evidence else []
    if args.evidence and not records:
        raise SystemExit('no actual result records found')
    samples = {}
    for path in records:
        sample = validate(path)
        validate(path.parent / 'config.json')
        samples[sample['schemaVersion']] = sample
    for sample in samples.values():
        mutations = [('providers', None), ('status', 'MAYBE'), ('unknown', True)]
        if 'revisionGate' in sample:
            mutations.append(('revisionGate', None))
        for key, value in mutations:
            invalid = copy.deepcopy(sample)
            invalid[key] = value
            try:
                validators[sample['schemaVersion']].validate(invalid)
            except ValidationError:
                continue
            raise SystemExit('invalid control unexpectedly accepted')
    print(f'{len(validators)} schemas and example configs valid; {len(records)} actual result/config pairs checked')


if __name__ == '__main__':
    main()
