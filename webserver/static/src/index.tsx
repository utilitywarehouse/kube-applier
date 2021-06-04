import React from 'react';
import ReactDOM from 'react-dom';
import { BrowserRouter, Switch, Route } from 'react-router-dom';
import Index from './components/Index';
import './index.css';

ReactDOM.render(
  <div className="m-auto w-2/3">
    <React.StrictMode>
      <BrowserRouter>
        <Switch>
          <Route path="/waybill/:id">
            <Index />
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
