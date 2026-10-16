export async function callGraphQL(query, variables, getAccessToken) {
    const token = await getAccessToken()
    const response = await fetch('http://localhost:9000/graphql', {method: 'POST', headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({query, variables}),
})
const result = await response.json();
return result
}