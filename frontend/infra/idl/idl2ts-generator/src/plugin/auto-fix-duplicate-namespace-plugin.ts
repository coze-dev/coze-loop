import path from 'path';

import { type Program, after, type IPlugin } from '@coze-arch/idl2ts-plugin';
import {
  type IParseEntryCtx,
  type IParseResultItem,
  isPbFile,
  isIdentifier,
  isMapType,
  isSetType,
  isListType,
  isStructDefinition,
  isServiceDefinition,
  isTypedefDefinition,
  isConstDefinition,
} from '@coze-arch/idl2ts-helper';
import type {
  FieldType,
  FunctionType,
  Identifier,
  StructDefinition,
} from '@coze-arch/idl-parser';

import { HOOK } from '../context';

/**
 * Fix TS2308 errors caused by multiple generated files exporting the same
 * namespace alias name from different source modules.
 *
 * The plugin prefixes namespace aliases for **cross-domain** includes with the
 * source domain directory name (e.g., `filter` → `observability_filter`), while
 * keeping **same-domain** includes unchanged.
 *
 * "Domain" is defined as the first divergent directory segment between the
 * importing file and the imported file. For example, files under
 * `evaluation/domain/` importing from `observability/domain/` cross the domain
 * boundary, so the alias gets the `observability_` prefix.
 */
export class AutoFixDuplicateNamespacePlugin implements IPlugin {
  apply(p: Program<{ PARSE_ENTRY: any }>) {
    p.register(after(HOOK.PARSE_ENTRY), (ctx: IParseEntryCtx) => {
      if (isPbFile(ctx.entries[0])) {
        return ctx;
      }

      // Collect all existing alias names across all AST items to detect conflicts
      const allAliases = new Set<string>();
      for (const astItem of ctx.ast) {
        for (const alias of Object.values(astItem.includeRefer)) {
          if (alias) {
            allAliases.add(alias);
          }
        }
      }

      // Cache for cross-domain renames: resolvedPath → newName.
      // Ensures the same target file gets a consistent prefixed name across
      // all files that cross-domain-include it, while NOT affecting same-domain
      // includes of the same file.
      const crossDomainRenameCache = new Map<string, string>();

      for (const astItem of ctx.ast) {
        for (const includeKey of Object.keys(astItem.includeMap)) {
          const alias = astItem.includeRefer[includeKey];
          if (!alias) {
            continue;
          }
          const resolvedPath = astItem.includeMap[includeKey];
          const targetDomain = getCrossDomain(astItem.idlPath, resolvedPath);

          if (!targetDomain) {
            // Same domain — keep original alias, no rename
            continue;
          }

          // Cross-domain include — check cache first
          const cached = crossDomainRenameCache.get(resolvedPath);
          if (cached !== undefined) {
            if (cached !== alias) {
              astItem.includeRefer[includeKey] = cached;
              renameNamespaceInStatements(astItem.statements, alias, cached);
              renameNamespaceInNestedDefs(astItem.statements, alias, cached);
            }
            continue;
          }

          // Prefix with target domain name
          let newName = `${targetDomain}_${alias}`;

          // If the prefixed name conflicts with an existing alias,
          // keep adding parent directory segments until unique
          if (allAliases.has(newName) && newName !== alias) {
            newName = deriveUniqueName(alias, resolvedPath, allAliases);
          }

          if (newName === alias) {
            crossDomainRenameCache.set(resolvedPath, alias);
            continue;
          }

          allAliases.add(newName);
          crossDomainRenameCache.set(resolvedPath, newName);
          astItem.includeRefer[includeKey] = newName;
          renameNamespaceInStatements(astItem.statements, alias, newName);
          renameNamespaceInNestedDefs(astItem.statements, alias, newName);
        }
      }

      return ctx;
    });
  }
}

/**
 * Determine whether two IDL file paths belong to different domains.
 *
 * Returns the target's domain directory name if they cross a domain boundary
 * (e.g., `"observability"`), or `null` if they are in the same domain.
 *
 * The domain boundary is the first divergent **directory** segment after the
 * common ancestor directory. If the paths only differ at the filename level
 * (same directory), they are considered same-domain.
 *
 * Both paths are normalized before comparison to avoid symlink/realpath
 * inconsistencies (e.g., /private/Users vs /Users on macOS).
 */
function getCrossDomain(sourcePath: string, targetPath: string): string | null {
  // Normalize paths to resolve symlinks, '..' etc. for stable comparison
  const sourceDir = path.dirname(path.resolve(sourcePath));
  const targetDir = path.dirname(path.resolve(targetPath));

  if (sourceDir === targetDir) {
    // Same directory — always same domain
    return null;
  }

  const sourceSegments = sourceDir.split(path.sep);
  const targetSegments = targetDir.split(path.sep);

  // Find the longest common prefix
  let commonLen = 0;
  const minLen = Math.min(sourceSegments.length, targetSegments.length);
  for (let i = 0; i < minLen; i++) {
    if (sourceSegments[i] === targetSegments[i]) {
      commonLen = i + 1;
    } else {
      break;
    }
  }

  // Get the first divergent directory segment for each path
  const sourceDomain = sourceSegments[commonLen];
  const targetDomain = targetSegments[commonLen];

  if (!sourceDomain || !targetDomain) {
    // One directory is a parent of the other (e.g., evaluation/ vs evaluation/domain/)
    // This means they are within the same module hierarchy
    return null;
  }

  if (sourceDomain === targetDomain) {
    // Same first-level directory after common prefix — same domain
    return null;
  }

  // Different domains — return the target domain name
  return targetDomain;
}

/**
 * Fallback: derive a unique alias name by trying parent directory segments
 * as prefix when the simple `domain_alias` conflicts with an existing name.
 * Generated names are sanitized to be valid JS identifiers.
 */
function deriveUniqueName(
  originalAlias: string,
  resolvedPath: string,
  usedNames: Set<string>,
): string {
  const dirSegments = path
    .dirname(resolvedPath)
    .split(path.sep)
    .filter(Boolean);

  // Try each individual parent directory as prefix (from closest to farthest)
  for (let i = dirSegments.length - 1; i >= 0; i--) {
    const candidate = sanitizeIdentifier(`${dirSegments[i]}_${originalAlias}`);
    if (!usedNames.has(candidate)) {
      return candidate;
    }
  }

  // Try combined prefixes with increasing depth
  for (let depth = 2; depth <= dirSegments.length; depth++) {
    const prefix = dirSegments.slice(dirSegments.length - depth).join('_');
    const candidate = sanitizeIdentifier(`${prefix}_${originalAlias}`);
    if (!usedNames.has(candidate)) {
      return candidate;
    }
  }

  // Fallback: append a numeric suffix
  let counter = 2;
  while (usedNames.has(`${originalAlias}_${counter}`)) {
    counter++;
  }
  return `${originalAlias}_${counter}`;
}

/**
 * Sanitize a string to be a valid JS identifier.
 * Replaces non-alphanumeric/non-underscore characters with underscores.
 */
function sanitizeIdentifier(name: string): string {
  return name.replace(/[^a-zA-Z0-9_$]/g, '_');
}

/**
 * Rename namespace references in all type identifiers within an array of statements.
 */
function renameNamespaceInStatements(
  statements: IParseResultItem['statements'],
  oldName: string,
  newName: string,
): void {
  for (const stmt of statements) {
    if (isStructDefinition(stmt)) {
      for (const field of stmt.fields) {
        renameInFieldType(field.fieldType, oldName, newName);
        if (field.defaultValue && isIdentifier(field.defaultValue as any)) {
          renameIdentifier(field.defaultValue as Identifier, oldName, newName);
        }
      }
    } else if (isServiceDefinition(stmt)) {
      for (const func of stmt.functions) {
        renameInFieldType(func.returnType as FieldType, oldName, newName);
        for (const field of func.fields) {
          renameInFieldType(field.fieldType, oldName, newName);
        }
        for (const field of func.throws) {
          renameInFieldType(field.fieldType, oldName, newName);
        }
      }
    } else if (isTypedefDefinition(stmt)) {
      renameInFieldType(stmt.definitionType, oldName, newName);
    } else if (isConstDefinition(stmt)) {
      renameInFieldType(stmt.fieldType, oldName, newName);
    }
  }
}

/**
 * Handle nested struct/enum definitions.
 */
function renameNamespaceInNestedDefs(
  statements: IParseResultItem['statements'],
  oldName: string,
  newName: string,
): void {
  for (const stmt of statements) {
    if (isStructDefinition(stmt) && stmt.nested) {
      for (const nestedDef of Object.values(stmt.nested)) {
        if (isStructDefinition(nestedDef as any)) {
          const nested = nestedDef as StructDefinition;
          for (const field of nested.fields) {
            renameInFieldType(field.fieldType, oldName, newName);
          }
        }
      }
    }
  }
}

/**
 * Recursively rename namespace in a field type.
 */
function renameInFieldType(
  fieldType: FieldType | FunctionType,
  oldName: string,
  newName: string,
): void {
  if (!fieldType) {
    return;
  }
  if (isIdentifier(fieldType as any)) {
    renameIdentifier(fieldType as Identifier, oldName, newName);
  } else if (isMapType(fieldType as any)) {
    const mapType = fieldType as { keyType: FieldType; valueType: FieldType };
    renameInFieldType(mapType.keyType, oldName, newName);
    renameInFieldType(mapType.valueType, oldName, newName);
  } else if (isListType(fieldType as any) || isSetType(fieldType as any)) {
    const containerType = fieldType as { valueType: FieldType };
    renameInFieldType(containerType.valueType, oldName, newName);
  }
}

/**
 * Rename namespace prefix in an Identifier node.
 */
function renameIdentifier(
  identifier: Identifier,
  oldName: string,
  newName: string,
): void {
  const prefix = `${oldName}.`;
  if (identifier.value.startsWith(prefix)) {
    identifier.value = `${newName}.${identifier.value.slice(prefix.length)}`;
  }
  if (identifier.namespaceValue?.startsWith(prefix)) {
    identifier.namespaceValue = `${newName}.${identifier.namespaceValue.slice(prefix.length)}`;
  }
}
