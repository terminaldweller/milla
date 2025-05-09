from datetime import datetime
from agents import function_tool


@function_tool
def fetch_time():
    """
    Fetches the current time.
    """

    return datetime.now().strftime("%H:%M:%S")


@function_tool
def fetch_date():
    """
    Fetches the current date.
    """

    return datetime.now().strftime("%Y-%m-%d")
