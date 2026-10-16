import { useState } from "react";
// import heroImg from './assets/hero.png'
// import reactLogo from './assets/react.svg'
// import viteLogo from './assets/vite.svg'
import "./App.css";
import { useAuth0 } from "@auth0/auth0-react";
import { callGraphQL } from "./api";

function App() {
  const [count, setCount] = useState(0);
  const [usage, setUsage] = useState(null);
  const [message, setMessage] = useState("");
  const [reply, setReply] = useState(null);
  const [error, setError] = useState(null);
  const {
    isAuthenticated,
    isLoading,
    loginWithRedirect,
    logout,
    getAccessTokenSilently,
  } = useAuth0();
  async function fetchUsage() {
    try {
      const data = await callGraphQL(USAGE_QUERY, {}, getAccessTokenSilently);
      if (data.errors) {
        setError(data.errors[0].message);
        return;
      }
      setError(null);
      setUsage(data.data.usage);
    } catch (err) {
      setError(err.message);
    }
  }

  async function sendMessage() {
    try {
      const data = await callGraphQL(
        SEND_MESSAGE_QUERY,
        { input: { model: "claude-haiku-4-5-20251001", content: message } },
        getAccessTokenSilently,
      );
      if (data.errors) {
        setError(data.errors[0].message);
        return;
      }
      setError(null);
      setReply(data.data.sendMessage);
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <>
      {isLoading ? (
        <p>Loading...</p>
      ) : isAuthenticated ? (
        <button
          onClick={() =>
            logout({ logoutParams: { returnTo: window.location.origin } })
          }
        >
          Log Out
        </button>
      ) : (
        <button onClick={() => loginWithRedirect()}>Log In</button>
      )}
      <button
        type="button"
        className="counter"
        onClick={() => setCount((count) => count + 1)}
      >
        Count is {count}
      </button>
      <button onClick={fetchUsage}>Fetch usage</button>
      {usage && (
        <p>
          Input: {usage.inputTokens} / Output: {usage.outputTokens}
        </p>
      )}
      <input
        type="text"
        value={message}
        onChange={(e) => setMessage(e.target.value)}
      />
      <button onClick={sendMessage}>Send</button>
      {reply && <p>{reply.content}</p>}
      {error && <p style={{ color: "red" }}>{error}</p>}
    </>
  );
}

const USAGE_QUERY = `
  query {
    usage {
      inputTokens
      outputTokens
    }
  }
`;

const SEND_MESSAGE_QUERY = `
  mutation SendMessage($input: SendMessageInput!) {
    sendMessage(input: $input) {
      id
      content
      stopReason
      usage {
        inputTokens
        outputTokens
      }
    }
  }
`;

export default App;
