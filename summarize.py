import argparse
import json
import sys

import torch
from transformers import pipeline


def summarize_text(text):
    # Check if a GPU is available
    device = 0 if torch.cuda.is_available() else -1

    # Create the summarization pipeline with the appropriate device
    summarizer = pipeline(
        "summarization", model="facebook/bart-large-cnn", device=device
    )

    # Add debugging information
    print(f"Text length: {len(text)}", file=sys.stderr)

    # Ensure the text is not empty
    if not text.strip():
        raise ValueError("Input text is empty")

    summary = summarizer(text, max_length=150, min_length=30, do_sample=False)

    # Add debugging information
    print(f"Summary: {summary}", file=sys.stderr)

    return summary[0]["summary_text"] if summary else None


def main():
    parser = argparse.ArgumentParser(
        description="Summarize text using a transformer model."
    )
    parser.add_argument("text", type=str, help="The text to summarize")
    args = parser.parse_args()

    response = {"summary": None, "error": None, "model_name": "facebook/bart-large-cnn"}

    try:
        summary = summarize_text(args.text)
        if summary:
            response["summary"] = summary
        else:
            response["error"] = "No summary could be generated."
            sys.exit(1)
    except Exception as e:
        response["error"] = f"An error occurred: {e}"
        sys.exit(1)
    finally:
        print(json.dumps(response))


if __name__ == "__main__":
    main()
