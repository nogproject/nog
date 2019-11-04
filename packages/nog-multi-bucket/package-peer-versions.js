import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

checkNpmVersions(
  {
    'aws-sdk': '2.411.x',
  },
  'nog-multi-bucket'
);
