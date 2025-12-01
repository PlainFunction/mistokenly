const { OpenAI } = require("openai");
const { Agent, run, tool } = require("@openai/agents");
const dotenv = require("dotenv");
const z = require("zod");

// ** NOTE: Remove console logs and handle errors appropriately in production code ** //

// Load environment variables from .env file 
dotenv.config();

// Get OpenAI API key from environment variables
const {OPENAI_API_KEY} = process.env;

// Define service variables
const SERVICE_URL = "http://localhost";
const REQUESTING_SERVICE = "Agent 1";

// ** Pull these from secure secret storage in a real application **
const ORGANIZATION_ID = "acme-corp";
const ORGANIZATION_KEY = "super-secret-key";
const CLIENT_ID = "client-123";

// Call the tokenisation service to tokenise PII data
const tokenize = async (data, dataType) => {
    const response = await fetch(`${SERVICE_URL}/v1/tokenize`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            data,
            dataType,
            clientId: CLIENT_ID,
            organizationId: ORGANIZATION_ID,
            organizationKey: ORGANIZATION_KEY
        })
    })

    if(!response.ok) {
        console.log(await response.json());
        throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json();
    return result.referenceHash;
}

// Call the tokenisation service to detokenise PII data
const detokenize = async (referenceHash) => {
    const response = await fetch(`${SERVICE_URL}/v1/detokenize`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            referenceHash,
            purpose: "Agent tool calling example",
            requestingService: REQUESTING_SERVICE,
            organizationId: ORGANIZATION_ID,
            organizationKey: ORGANIZATION_KEY
        })
    });

    if(!response.ok) {
        console.log(await response.json());
        throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json();
    return result.data;
}

// Example function that uses detokenization to get the email and "sends" an email
const sendEmailFunction = async (email_reference_hash) => {
    console.log("Sending email to reference hash:", email_reference_hash);
    const email = await detokenize(email_reference_hash);
    console.log("Revealed email address:", email);

    const subject = "Hello from the agentic tool-calling example!";
    return `Email sent with subject "${subject}".`;
}

// Define the tool that the agent can use to send an email
const sendEmailTool = tool({
  name: 'send_email',
  description: 'Send an email to a given address',
  parameters: z.object({ email_reference_hash: z.string().regex(/^tok_[a-fA-F0-9]{32}$/, "Invalid token format") }),
  async execute({ email_reference_hash }) {
    const result = await sendEmailFunction(email_reference_hash);
    return result;
  },
});

// Initialize OpenAI client
const openai = new OpenAI({ apiKey: OPENAI_API_KEY });

// Create an agent and make a tool call using @openai/agents
async function runAgent() {
    const agent = new Agent({
        name: REQUESTING_SERVICE,
        model: "gpt-4.1-nano",
        client: openai,
        tools: [sendEmailTool]
    });

    // Example prompt containing PII (phone and email address)
    const promptExample = "Hello there! My phone number is 0412345678 and my email is user@example.com";

    // Example basic regex rules for different PII types - expand as needed, or use a dedicated PII detection library/service
    const piiRegexRules = {
        email: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b/g,
        phone: /\b(?:\+?\d{1,3}[-.\s]?)?(?:\(?\d{3}\)?[-.\s]?)?\d{3}[-.\s]?\d{4}\b/g,
        ssn: /\b\d{3}-\d{2}-\d{4}\b/g,
        creditCard: /\b(?:\d[ -]*?){13,16}\b/g,
        ip: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g
    };

    // Store reference hash for PII found in the prompt - in this example, just the email, but this could be a map for multiple types
    var user_email_reference_hash;

    // Dynamically obscure PII in the prompt before sending to the agent
    await Promise.all(Object.keys(piiRegexRules).map(async (dataType) => {
        const regex = piiRegexRules[dataType];
        const matches = promptExample.match(regex);
        if (matches) {
            await tokenize(matches[0], dataType).then((referenceHash) => {
                console.log(`Replaced ${dataType} "${matches[0]}" with reference hash: ${referenceHash}`);
                if (dataType === "email") {                    
                    user_email_reference_hash = referenceHash;
                }
            });
        }
    }));

    console.log("Email reference hash:", user_email_reference_hash);

    // Run the agent with the modified prompt containing reference hashes -- the agent DOES NOT see any raw PII
    const response = await run(agent, `Send an email to ${user_email_reference_hash}`);

    console.log("Agent response:", response.finalOutput);
}

runAgent();
