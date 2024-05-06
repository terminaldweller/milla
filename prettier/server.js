#!/usr/bin/env node
"use strict";

const express = require("express");
const prettier = require("prettier");

const app = express();
app.use(express.json());

// app.use((req, res, next) => {
//   res.append("Access-Control-Allow-Origin", ["*"]);
//   res.append("Access-Control-Allow-Methods", "POST", "OPTIONS");
//   res.append("Access-Control-Allow-Headers", "Content-Type");
//   next();
// });

app.post("/api/v1/format", async function (req, res) {
  const formattedText = await prettier.format(req.body["content"], {
    parser: "babel",
  });
  res.set("Content-Type", "application/json");
  res.json({ formattedText: formattedText });
});

app.listen(9001);
