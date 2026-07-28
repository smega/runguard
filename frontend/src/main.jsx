import { render } from "preact";
import App from "./App.jsx";
import { NotifyProvider } from "./components/notify.jsx";
import "./index.css";

render(
  <NotifyProvider>
    <App />
  </NotifyProvider>,
  document.getElementById("app")
);



