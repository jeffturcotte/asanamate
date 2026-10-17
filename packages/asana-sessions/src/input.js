// readBounded reads a whole stream as UTF-8, failing once it exceeds max bytes.
export async function readBounded(stream, max, tooLarge) {
  let size = 0;
  const chunks = [];
  for await (const data of stream) {
    const chunk = Buffer.from(data);
    size += chunk.length;
    if (size > max) {
      throw tooLarge();
    }
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf8');
}
