export type DiffLine = { left?: string; right?: string; changed: boolean };

export function jsonFields(text: string): string[] | null {
  try {
    const parsed: unknown = JSON.parse(text);
    const fields: string[] = [];
    function visit(value: unknown, path: string): void {
      if (value !== null && typeof value === 'object') {
        const entries = Object.entries(value);
        if (entries.length) {
          for (const [key, child] of entries) visit(child, path + '/' + key.replace(/~/g, '~0').replace(/\//g, '~1'));
          return;
        }
      }
      fields.push(`${path || '/'} = ${JSON.stringify(value)}`);
    }
    visit(parsed, '');
    return fields.sort();
  } catch {
    return null;
  }
}

export function alignLines(left: string[], right: string[]): DiffLine[] | null {
  if (left.length > 600 || right.length > 600) return null;
  const width = right.length + 1;
  const matches = new Uint16Array((left.length + 1) * width);
  for (let a = left.length - 1; a >= 0; a--) {
    for (let b = right.length - 1; b >= 0; b--) {
      const i = a * width + b;
      matches[i] = left[a] === right[b] ? 1 + matches[i + width + 1] : Math.max(matches[i + width], matches[i + 1]);
    }
  }
  const rows: DiffLine[] = [];
  let a = 0;
  let b = 0;
  while (a < left.length || b < right.length) {
    if (a < left.length && b < right.length && left[a] === right[b]) {
      rows.push({ left: left[a++], right: right[b++], changed: false });
    } else if (a < left.length && (b === right.length || matches[(a + 1) * width + b] >= matches[a * width + b + 1])) {
      rows.push({ left: left[a++], changed: true });
    } else {
      rows.push({ right: right[b++], changed: true });
    }
  }
  return rows;
}
