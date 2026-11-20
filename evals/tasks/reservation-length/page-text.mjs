// The text of an HTML page as a person reads it, for the grader of the task
// reservation-length (evals/): it reads the HTML on stdin and writes its text
// on one line.
//
// The HTML that Next.js streams splits the text of a component into nodes
// with empty comments ("1<!-- --> h"), holds the parts that streamed in late
// in hidden elements, and carries the same content again as data in scripts.
// So the scripts, the styles, and the comments go; every tag becomes a space;
// the entities are decoded; and every run of white space, a no-break space
// included, becomes one space.
import { text } from "node:stream/consumers";

const named = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: " " };

const html = await text(process.stdin);
const page = html
  .replace(/<script\b[\s\S]*?<\/script>/gi, " ")
  .replace(/<style\b[\s\S]*?<\/style>/gi, " ")
  .replace(/<!--[\s\S]*?-->/g, "")
  .replace(/<[^>]*>/g, " ")
  .replace(/&(#x[0-9a-f]+|#[0-9]+|[a-z]+);/gi, (entity, name) => {
    if (name.startsWith("#x") || name.startsWith("#X")) {
      return String.fromCodePoint(parseInt(name.slice(2), 16));
    }
    if (name.startsWith("#")) {
      return String.fromCodePoint(parseInt(name.slice(1), 10));
    }
    return named[name.toLowerCase()] ?? entity;
  })
  .replace(/[\s  ]+/g, " ")
  .trim();
process.stdout.write(`${page}\n`);
