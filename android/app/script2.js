const fs = require('fs');

const data = fs.readFileSync('C:/Users/PC/.gemini/antigravity/brain/1445e0bd-b2e3-41f4-ac18-5849faa3dc00/.system_generated/logs/transcript.jsonl', 'utf8');
const lines = data.split('\n');

for (const line of lines) {
    if (!line) continue;
    try {
        const obj = JSON.parse(line);
        if (obj.type === 'TOOL_RESPONSE' || obj.type === 'VIEW_FILE') {
            if (obj.content && obj.content.includes('MainActivity.kt')) {
                console.log('Found MainActivity.kt response, length:', obj.content.length);
                if (obj.content.includes('Showing lines')) {
                    const match = obj.content.match(/Showing lines (\d+) to (\d+)/);
                    if (match) {
                        console.log('Lines:', match[1], 'to', match[2]);
                    }
                }
            }
        }
    } catch(e) { }
}
