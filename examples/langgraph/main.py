# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "langgraph",
#     "requests"
# ]
# ///


from typing import TypedDict

from langgraph.graph import StateGraph, START, END
from langchain_core.runnables import RunnableLambda
import requests


class State(TypedDict):
    query: str
    response: str


def _call_api_bridge_agent(service: str, query: str) -> str:
    print(f"🟡 Asking the following question to the API Bridge Agnt({service}): {query}")

    headers = {
        "Accept": None,
        "Accept-Encoding": None,
        "Content-Type": "application/nlq",
    }
    r = requests.post(f"http://localhost:8080/{service}/", headers=headers, data=query)
    r.raise_for_status()
    response = r.text

    print(f"🟡 API Bridge Agnt answered with: {response}")

    return response


def call_api_bridge_agent(_dict) -> str:
    return _call_api_bridge_agent(_dict["service"], _dict["query"])


api_bridge_runnable = RunnableLambda(call_api_bridge_agent)


def github_commits_node(state: State):
    response = api_bridge_runnable.invoke(
        {
            "service": "github",
            "query": state["query"],
        }
    )

    return {"response": response}


def main():
    graph_builder = StateGraph(State)
    graph_builder.add_node(github_commits_node)

    graph_builder.add_edge(START, "github_commits_node")
    graph_builder.add_edge("github_commits_node", END)

    graph = graph_builder.compile()

    new_state = graph.invoke(
        {"query": "Give me the last 3 commits of the agntcy/docs repository"}
    )

    print(f"🟡 New workflow state: {new_state}")


if __name__ == "__main__":
    main()
