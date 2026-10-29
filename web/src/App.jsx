import { useState } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import { callGraphQL } from "./api";

function App() {
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

  if (isLoading) {
    return <p>Loading...</p>;
  }

  if (!isAuthenticated) {
    return (
      <div className="min-h-screen flex">
        <div className="w-1/2 bg-black flex flex-col justify-center px-1 animate-fade-in">
          <h1 className="text-4xl font-serif text-white mb-2">Sign In</h1>
          <p className="text-gray-400 mb-8">
            Welcome to sift — log in to continue.
          </p>
          <button
            onClick={() => loginWithRedirect()}
            className="bg-purple-600 hover:bg-purple-700 text-white font-medium px-8 py-3 rounded-full w-fit"
          >
            Log In
          </button>
        </div>
        <div
          className="w-1/2 bg-gradient-to-br from-purple-600 to-purple-900 animate-fade-in"
          style={{ animationDelay: "0.2s" }}
        ></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-black flex items-center justify-center p-4">
      <div className="bg-gray-900 rounded-lg shadow-lg p-6 max-w-md w-full flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-serif text-white">sift</h1>

          <button
            onClick={() =>
              logout({ logoutParams: { returnTo: window.location.origin } })
            }
            className="text-gray-400 hover:text-white text-sm"
          >
            Log Out
          </button>
        </div>
        <button
          className="bg-purple-600 text-white rounded-full px-4 py-2 hover:bg-purple-700"
          onClick={fetchUsage}
        >
          Fetch usage
        </button>
        {usage && (
          <p className="text-gray-400">
            Input: {usage.inputTokens} / Output: {usage.outputTokens}
          </p>
        )}
        <input
          className="border border-gray-700 bg-gray-800 text-white rounded-md px-3 py-2"
          type="text"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
        />
        <button
          className="bg-purple-600 text-white rounded-full px-4 py-2 hover:bg-purple-700"
          onClick={sendMessage}
        >
          Send
        </button>
        {reply && <p className="text-gray-200">{reply.content}</p>}
        {error && <p className="text-red-400">{error}</p>}
      </div>
    </div>
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
