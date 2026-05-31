const fs = require('fs');

const data = fs.readFileSync('C:/Users/PC/.gemini/antigravity/brain/1445e0bd-b2e3-41f4-ac18-5849faa3dc00/.system_generated/logs/transcript.jsonl', 'utf8');
const lines = data.split('\n');

for (const line of lines) {
    if (!line) continue;
    try {
        const obj = JSON.parse(line);
        if (obj.content && obj.content.includes('Showing lines 1 to 800') && obj.content.includes('MainActivity.kt')) {
            const text = obj.content;
            const startIdx = text.indexOf('The following code has been modified');
            // use regex to split by \r\n or \n
            const linesPart = text.substring(startIdx).split(/\r?\n/);
            let reconstructed = [];
            let started = false;
            for (const l of linesPart) {
                if (l.startsWith('The following code')) {
                    started = true;
                    continue;
                }
                if (l.startsWith('The above content')) {
                    break;
                }
                if (started) {
                    // Remove line prefix '123: '
                    const match = l.match(/^\d+:\s?(.*)/);
                    if (match) {
                        reconstructed.push(match[1]);
                    } else {
                        // Sometimes there's no prefix if it's wrapped, but let's just push it
                        reconstructed.push(l);
                    }
                }
            }
            
            fs.writeFileSync('C:/Users/PC/Desktop/MainActivity_part1.txt', reconstructed.join('\n'));
            console.log('Saved ' + reconstructed.length + ' lines');
            break;
        }
    } catch(e) { }
}
