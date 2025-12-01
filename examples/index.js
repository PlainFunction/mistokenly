const tokenize = async (data) => {
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
        })
    })

    if(!response.ok) {
        console.log(response)
        console.log(await response.json());
        throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json();
    return result;
}

const detokenize = async (referenceHash) => {
    const response = await fetch("http://localhost/v1/detokenize", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            referenceHash,
            purpose: "customer-service",
            requestingService: "crm",
            requestingUser: "user@example.com",
            organizationId: "acme-corp",
            organizationKey: "super-secret-key"
        })
    });

    if(!response.ok) {
        console.log(response);
        console.log(await response.json());
        throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json();
    return result;
}

// Attempt to tokenize and then detokenize a sample string
tokenize("hello there!").then(async result => {
    console.log("Tokenization result:", result);

    await new Promise(resolve => setTimeout(resolve, 1000)); // Wait for 1 second

    // Extract the token from the result
    const token = result.referenceHash;
    
    // Now detokenize using the new token
    return detokenize(token);
}).then(result => {
    console.log("Detokenization result:", result);
}).catch(err => {
    console.error("Error:", err);
});

