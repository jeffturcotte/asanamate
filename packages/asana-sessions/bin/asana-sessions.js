#!/usr/bin/env node
import { main } from '../src/cli.js';

// Exit once output is flushed, even if a timed-out provider call still runs.
const code = await main(process.argv.slice(2));
process.stdout.write('', () => process.exit(code));
