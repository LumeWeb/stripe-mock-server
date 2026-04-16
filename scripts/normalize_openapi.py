#!/usr/bin/env python3
"""
Normalize OpenAPI spec by converting inline enum definitions into standalone component schemas.

Fixes Go code generation conflicts from:
1. Conflicting enum values in same-named properties
2. anyOf patterns with inline enum array items causing duplicate type declarations

Optimizations:
- Single-pass traversal for both conversion types
- Hash-based fingerprinting (faster than MD5)
- Minimal string operations
- Efficient data structures
"""

import json
import sys
import subprocess

def ensure_components_section(spec: dict):
    """Ensure components.schemas exists."""
    if 'components' not in spec:
        spec['components'] = {}
    if 'schemas' not in spec['components']:
        spec['components']['schemas'] = {}

def get_enum_hash(enum_def: dict) -> int:
    """Fast hash-based fingerprint for enum definition."""
    enum_type = enum_def.get('type', 'string')
    enum_tuple = tuple(sorted([str(v) for v in enum_def.get('enum', [])]))
    return hash((enum_type, enum_tuple))

def get_schema_hash(schema: dict) -> int:
    """Fast hash-based fingerprint for object schema."""
    # Create a stable string representation of the schema
    schema_copy = {k: v for k, v in schema.items() if k != 'title'}
    return hash(json.dumps(schema_copy, sort_keys=True))

def normalize_title(title: str) -> str:
    """Normalize a title to a valid schema name (snake_case)."""
    # Convert PascalCase/camelCase to snake_case
    result = []
    for i, c in enumerate(title):
        if c.isupper() and i > 0 and title[i-1].islower():
            result.append('_')
        elif c.isupper() and i > 0 and i < len(title) - 1 and title[i+1].islower():
            result.append('_')
        result.append(c.lower())
    return ''.join(result)

def traverse_spec(spec: dict) -> tuple:
    """
    Single-pass traversal finding:
    1. Form data enums with potential conflicts
    2. anyOf array items with enums
    3. Inline object schemas with titles
    
    Returns:
        (formdataenums_by_path, anyofarrayenums_by_hash, inline_objects_by_title)
    """
    formdata_enums = {}  # {path: [enum_infos]}
    anyof_enums = {}     # {hash: [enum_infos]}
    inline_objects = {}  # {title: [schema_infos]}
    
    def is_formdata_context(path_parts) -> bool:
        """Check if current path is within a form data schema."""
        if len(path_parts) < 3:
            return False
        # Check if we're under content -> application/x-www-form-urlencoded
        for i in range(len(path_parts) - 2):
            if path_parts[i] == 'content' and path_parts[i+1] == 'application/x-www-form-urlencoded':
                return True
        return False
    
    def is_component_schema_definition(path_parts) -> bool:
        """Check if current path is a component schema definition (not a nested property)."""
        # We're at a component schema definition if:
        # path is ['root', 'components', 'schemas', '<schema_name>']
        # or ['components', 'schemas', '<schema_name>'] (after initial root)
        if len(path_parts) < 4:
            return False
        # Check if path pattern matches components/schemas/<name>
        # We're at the schema definition level if the path ends right after the schema name
        # i.e., the last two elements before the schema name are 'components' and 'schemas'
        # and we're at the schema name itself
        if path_parts[-3] == 'components' and path_parts[-2] == 'schemas':
            # We're at the schema name level or deeper
            # We only want to skip if we're AT the schema name level
            # If we're at properties/<prop>, path_parts would be [..., 'schemas', '<name>', 'properties', '<prop>']
            # So path_parts[-3] would be 'schemas' and path_parts[-2] would be '<name>'
            return len(path_parts) == 4 or (len(path_parts) > 4 and path_parts[-4] != 'schemas')
        return False
    
    def scan(obj, parent, key, path_parts):
        if not isinstance(obj, dict):
            return
        
        # Add current key to path
        path_parts.append(key)
        
        # Check for form data enums
        if 'enum' in obj and is_formdata_context(path_parts):
            enum_info = {
                'schema': obj,
                'parent_schema': parent,
                'path': '.'.join(str(p) for p in path_parts)
            }
            path_key = enum_info['path']
            if path_key not in formdata_enums:
                formdata_enums[path_key] = []
            formdata_enums[path_key].append(enum_info)
        
        # Check for anyOf array items with enums
        if 'anyOf' in obj:
            for branch in obj['anyOf']:
                if isinstance(branch, dict) and branch.get('type') == 'array':
                    items = branch.get('items')
                    if isinstance(items, dict) and 'enum' in items:
                        enum_info = {
                            'branch': branch,
                            'items': items,
                            'parent': obj
                        }
                        enum_hash = get_enum_hash(items)
                        if enum_hash not in anyof_enums:
                            anyof_enums[enum_hash] = []
                        anyof_enums[enum_hash].append(enum_info)
        
        # Check for inline object schemas with titles (skip component schema definitions themselves)
        if ('title' in obj and obj.get('type') == 'object' and 
            not is_component_schema_definition(path_parts) and
            '$ref' not in obj):
            title = obj['title']
            schema_info = {
                'schema': obj,
                'parent': parent,
                'key_in_parent': key
            }
            if title not in inline_objects:
                inline_objects[title] = []
            inline_objects[title].append(schema_info)
        
        # Continue traversal (make copy for each branch)
        for k, v in obj.items():
            if isinstance(v, list):
                for i, item in enumerate(v):
                    scan(item, obj, f"{k}[{i}]", list(path_parts))
            else:
                scan(v, obj, k, list(path_parts))
    
    scan(spec, None, 'root', [])
    return formdata_enums, anyof_enums, inline_objects

def process_formdata_enums(spec: dict, formdata_enums: dict) -> dict:
    """Process and convert form data inline enums to component schemas."""
    ensure_components_section(spec)
    
    conflicts = {k: v for k, v in formdata_enums.items() if len(v) > 1}
    converted_count = 0
    skipped_count = 0
    schema_cache = {}
    
    for path, enum_infos in formdata_enums.items():
        if len(enum_infos) <= 1:
            skipped_count += 1
            continue
        
        for enum_info in enum_infos:
            enum_def = enum_info['schema']
            enum_hash = get_enum_hash(enum_def)
            
            # Generate stable schema name
            parts = path.split('.')
            property_name = parts[-1] if parts else 'unknown'
            clean_name = property_name.replace('.', '/').replace('_', '/')
            schema_name = f"{clean_name}_enum_{abs(enum_hash)}"
            
            # Cache and reuse schema names for identical enums
            if enum_hash in schema_cache:
                schema_name = schema_cache[enum_hash]
            else:
                schema_cache[enum_hash] = schema_name
                spec['components']['schemas'][schema_name] = {
                    'type': enum_def.get('type', 'string'),
                    'enum': enum_def['enum']
                }
                converted_count += 1
            
            # Replace inline enum with $ref
            enum_def.clear()
            enum_def['$ref'] = f"#/components/schemas/{schema_name}"
    
    return {
        'found': sum(len(v) for v in formdata_enums.values()),
        'conflicts': len(conflicts),
        'converted': converted_count,
        'skipped': skipped_count
    }

def process_anyof_enums(spec: dict, anyof_enums: dict) -> dict:
    """Process and convert anyOf array items with enums to component schemas."""
    ensure_components_section(spec)
    
    schemas_added = 0
    converted_count = 0
    
    for enum_hash, items_list in anyof_enums.items():
        schema_name = f"anyof_array_items_{abs(enum_hash)}"
        
        if schema_name not in spec['components']['schemas']:
            template = items_list[0]['items'].copy()
            spec['components']['schemas'][schema_name] = template
            schemas_added += 1
        
        for item in items_list:
            item['branch']['items'] = {"$ref": f"#/components/schemas/{schema_name}"}
            converted_count += 1
    
    return {
        'found': sum(len(v) for v in anyof_enums.values()),
        'unique': len(anyof_enums),
        'converted': converted_count,
        'schemas_added': schemas_added
    }

def process_inline_objects(spec: dict, inline_objects: dict) -> dict:
    """Process and convert inline object schemas with titles to component schemas."""
    ensure_components_section(spec)
    
    schemas_added = 0
    converted_count = 0
    skipped_count = 0
    
    for title, schema_infos in inline_objects.items():
        schema_name = normalize_title(title)
        
        # Check if schema already exists in components
        if schema_name in spec['components']['schemas']:
            # Schema exists, just add refs
            pass
        else:
            # Add new component schema (use first occurrence as template)
            template = schema_infos[0]['schema'].copy()
            # Remove title from component schema (it's now the schema name)
            template.pop('title', None)
            spec['components']['schemas'][schema_name] = template
            schemas_added += 1
        
        # Replace all inline schemas with $ref
        for info in schema_infos:
            parent = info['parent']
            key = info['key_in_parent']
            if isinstance(parent, dict) and key in parent:
                parent[key] = {"$ref": f"#/components/schemas/{schema_name}"}
                converted_count += 1
    
    return {
        'found': sum(len(v) for v in inline_objects.values()),
        'unique': len(inline_objects),
        'converted': converted_count,
        'schemas_added': schemas_added,
        'skipped': skipped_count
    }

def validate_json(file_path: str) -> bool:
    """Validate JSON file."""
    try:
        with open(file_path, 'r') as f:
            json.load(f)
        return True
    except (json.JSONDecodeError, IOError):
        return False

def main():
    if len(sys.argv) < 2:
        sys.stderr.write("ERROR: Usage: python fix_openapi_enums.py <spec_file> [output_file]\n")
        sys.exit(1)
    
    spec_file = sys.argv[1]
    output_file = sys.argv[2] if len(sys.argv) > 2 else spec_file
    
    # Validate input
    if not validate_json(spec_file):
        sys.stderr.write(f"ERROR: Input file {spec_file} is not valid JSON\n")
        sys.exit(1)
    
    # Load spec
    try:
        with open(spec_file, 'r') as f:
            spec = json.load(f)
    except Exception as e:
        sys.stderr.write(f"ERROR: Failed to load spec: {e}\n")
        sys.exit(1)
    
    # Single-pass traversal
    formdata_enums, anyof_enums, inline_objects = traverse_spec(spec)
    
    # Process conversions
    try:
        formdata_results = process_formdata_enums(spec, formdata_enums)
        anyof_results = process_anyof_enums(spec, anyof_enums)
        inline_results = process_inline_objects(spec, inline_objects)
    except Exception as e:
        sys.stderr.write(f"ERROR: Processing failed: {e}\n")
        sys.exit(1)
    
    # Write output
    try:
        with open(output_file, 'w') as f:
            json.dump(spec, f, indent=2)
    except Exception as e:
        sys.stderr.write(f"ERROR: Failed to write output: {e}\n")
        sys.exit(1)
    
    # Validate output
    if not validate_json(output_file):
        sys.stderr.write(f"ERROR: Output file {output_file} is not valid JSON\n")
        sys.exit(1)
    
    # jq validation (silent, non-blocking)
    try:
        subprocess.run(
            ['jq', '.', output_file],
            capture_output=True,
            timeout=30,
            check=True
        )
    except (FileNotFoundError, subprocess.TimeoutExpired, subprocess.CalledProcessError):
        pass
    
    # Output summary
    print(json.dumps({
        'status': 'success',
        'input_file': spec_file,
        'output_file': output_file,
        'formdata_processing': formdata_results,
        'anyof_processing': anyof_results,
        'inline_object_processing': inline_results,
        'total_component_schemas': len(spec.get('components', {}).get('schemas', {}))
    }, indent=2))

if __name__ == '__main__':
    main()