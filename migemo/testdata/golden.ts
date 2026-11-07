// Writes golden.json from jsmigemo, the library kensaku.vim runs on. Patterns are
// kept as digests because the dictionary, and so the patterns, are GPL.
//
//   deno run -A migemo/testdata/golden.ts <migemo-compact-dict> > migemo/testdata/golden.json
import * as jsmigemo from "npm:jsmigemo@0.4.8";
import { crypto } from "jsr:@std/crypto@1";
import { encodeHex } from "jsr:@std/encoding@1/hex";

const queries = [
  "a", "k", "n", "ka", "ky", "sh", "tsu", "nihongo", "nihon", "nihonn",
  "kensaku", "kanji", "katakana", "hiragana", "tokyo", "toukyou", "kyouto",
  "osaka", "oosaka", "gakkou", "kitte", "matcha", "shinkansen", "konnichiha",
  "arigatou", "sayounara", "ohayou", "tenki", "kaigi", "shiryou", "sakura",
  "fuji", "yama", "kawa", "umi", "denwa", "bangou", "jikan", "benkyou",
  "Nihongo", "NihonGo", "KENSAKU", "HogeFuga", "a b", "foo.bar", "c++",
  "日本", "にほん", "カタカナ", "ﾆﾎﾝ", "ｎｉｈｏｎ", "100", "x-y", "(paren)", "[z",
];

const vim = { prefix: "\\m", or: "\\|", startGroup: "\\%(", endGroup: "\\)", startClass: "[", endClass: "]", newline: "", escape: "\\.[]*~^$" };
const javascript = { or: "|", startGroup: "(?:", endGroup: ")", startClass: "[", endClass: "]", newline: "", escape: "\\.[]{}()*+-?^$|" };
const romanTable: [string, string, number?][] = [
  ["qq", "っ", 1], ["kz", "かん", 0], ["ka", "か"], ["ki", "き"], ["sz", "さん"], ["n", "ん"], ["na", "な"], ["no", "の"],
];

const sha = async (s: string) =>
  encodeHex(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s)));
// Neovim receives the patterns as UTF-8, so a lone surrogate arrives as U+FFFD.
const utf8 = (s: string) => new TextDecoder().decode(new TextEncoder().encode(s));

const data = Deno.readFileSync(Deno.args[0]);
const dict = new jsmigemo.CompactDictionary(data.buffer);

function run(m: jsmigemo.Migemo, query: string, rxop: typeof javascript & { prefix?: string }) {
  m.setRxop([rxop.or, rxop.startGroup, rxop.endGroup, rxop.startClass, rxop.endClass, rxop.newline, rxop.escape]);
  return utf8((rxop.prefix ?? "") + m.query(query));
}

async function table(m: jsmigemo.Migemo) {
  const out = [];
  for (const query of queries) {
    const v = run(m, query, vim);
    const j = run(m, query, javascript);
    out.push({ query, vim: await sha(v), vimLength: v.length, javascript: await sha(j), javascriptLength: j.length });
  }
  return out;
}

const plain = new jsmigemo.Migemo();
plain.setDict(dict);
const custom = new jsmigemo.Migemo();
custom.setDict(dict);
custom.setRomajiProcessor(new jsmigemo.RomajiProcessor1(romanTable.map(([a, b, c]) => new jsmigemo.RomanEntry(a, b, c ?? 0))));

// Every romaji up to four characters, as lines of "romaji prefix suffix,suffix".
const processor = jsmigemo.RomajiProcessor2.build();
const alphabet = "abcdefghijklmnopqrstuvwxyz-~.,/[]'";
const lines: string[] = [];
function walk(s: string, depth: number) {
  if (s) {
    const r = processor.romajiToHiraganaPredictively(s);
    lines.push(`${s} ${r.prefix} ${[...new Set(r.suffixes)].sort().join(",")}`);
  }
  if (depth > 0) for (const c of alphabet) walk(s + c, depth - 1);
}
walk("", 4);

console.log(JSON.stringify({
  plain: await table(plain),
  romanTable,
  custom: await table(custom),
  alphabet,
  romajiDigest: await sha(lines.join("\n")),
}, null, 1));
