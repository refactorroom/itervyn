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
    for name in ['config', 'result', 'diagnostic', 'doctor']:
        schema = json.loads((root / 'schemas' / f'{name}-v1alpha1.schema.json').read_text())
        Draft202012Validator.check_schema(schema)
        validators[name] = Draft202012Validator(schema, format_checker=FormatChecker())
    validators['config'].validate(json.loads((root / 'examples/go-project/itervyn.json').read_text()))
    records = sorted(args.evidence.rglob('result.json')) if args.evidence else []
    if args.evidence and not records:
        raise SystemExit('no actual result records found')
    for path in records:
        validators['result'].validate(json.loads(path.read_text()))
        validators['config'].validate(json.loads((path.parent / 'config.json').read_text()))
    if records:
        sample = json.loads(records[-1].read_text())
        for key, value in [('providers', None), ('status', 'MAYBE'), ('unknown', True)]:
            invalid = copy.deepcopy(sample)
            invalid[key] = value
            try:
                validators['result'].validate(invalid)
            except ValidationError:
                continue
            raise SystemExit('invalid control unexpectedly accepted')
    print(f'4 schemas and example valid; {len(records)} actual result/config pairs checked')


if __name__ == '__main__':
    main()
