import { ReactiveVar } from 'meteor/reactive-var';
import { Template } from 'meteor/templating';
import './nog-flow-ui-more.html';


Template.nogOneWorkspaceResult.onCreated(function onCreated() {
  this.visible = new ReactiveVar(false);
});

Template.nogOneWorkspaceResult.helpers({
  isVisible() {
    const tpl = Template.instance();
    return tpl.visible.get();
  },
});

Template.nogOneWorkspaceResult.events({
  'show.bs.collapse .collapse'() {
    const tpl = Template.instance();
    tpl.visible.set(true);
  },
  'hide.bs.collapse .collapse'() {
    const tpl = Template.instance();
    tpl.visible.set(false);
  },
});
