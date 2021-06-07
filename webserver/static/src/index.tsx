import React from 'react';
import ReactDOM from 'react-dom';
import { BrowserRouter, Switch, Route } from 'react-router-dom';
import Index from './components/Index';
import Namespace from './components/Namespace';
import './index.css';

ReactDOM.render(
  <div className="m-auto py-8 w-2/3">
    <h1 className="text-center text-6xl mb-8 font-extrabold text-transparent bg-clip-text bg-gradient-to-tr from-indigo-700 to-indigo-600">kube-applier</h1>
    <React.StrictMode>
      <BrowserRouter>
        <Switch>
          <Route path="/:namespace">
            <Namespace />
          </Route>
          <Route path="/">
            <Index />
          </Route>
        </Switch>
      </BrowserRouter>
    </React.StrictMode>
  </div>,
  document.getElementById('root')
);
