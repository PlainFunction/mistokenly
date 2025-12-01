const http = require('http');

const tokenize = async (data) => {
    const agent = new http.Agent({ keepAlive: false });
    const response = await fetch("http://localhost/v1/tokenize", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            data,
            dataType: "email",
            clientId: "client-123",
            organizationId: "acme-corp",
            organizationKey: "super-secret-key"
        }),
        agent
    })

    if(!response.ok) {
        console.log(await response.json());
        throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json();
    return result;
}

const spamTokenize = async (data, count = 100) => {
    const requests = [];
    for (let i = 0; i < count; i++) {
        requests.push(tokenize(data));
    }
    return Promise.allSettled(requests);
};

spamTokenize("This is a spam email! Buy now!", 500).then(results => {
    results.forEach((result, index) => {
        console.log(result)
        if (result.status === "fulfilled") {
            console.log(`Tokenization result ${index + 1}:`, result.value.status);
        } else {
            console.error(`Error during tokenization ${index + 1}:`, result.message);
        }
    });
}).catch(err => {
    console.error("Unexpected error during spam tokenization:", err);
});