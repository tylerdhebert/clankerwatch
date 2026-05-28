export function highlightSQL(value: string) {
  const escaped = escapeHTML(value);
  return escaped
    .replace(/(--.*$)/gim, '<span class="sql-comment">$1</span>')
    .replace(/('[^']*')/g, '<span class="sql-string">$1</span>')
    .replace(/\b(select|from|where|join|left|right|inner|outer|on|with|as|and|or|order|by|group|limit|offset|case|when|then|else|end|explain|having|union|distinct)\b/gi, '<span class="sql-keyword">$1</span>')
    .replace(/\b(\d+)\b/g, '<span class="sql-number">$1</span>');
}

function escapeHTML(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
