const fs = require('fs');

const data = fs.readFileSync('C:/Users/PC/.gemini/antigravity/brain/1445e0bd-b2e3-41f4-ac18-5849faa3dc00/.system_generated/logs/transcript.jsonl', 'utf8');
const lines = data.split('\n');

for (const line of lines) {
    if (!line) continue;
    try {
        const obj = JSON.parse(line);
        if (obj.type === 'RUN_COMMAND' || obj.type === 'TOOL_RESPONSE') {
            if (obj.content && obj.content.includes('LaunchedEffect(sessionToken)') && obj.content.includes('processWakeIntent')) {
                console.log('Found git diff response, length:', obj.content.length);
                fs.writeFileSync('C:/Users/PC/Desktop/git_diff_recovery.txt', obj.content);
            }
        }
    } catch(e) { }
}
