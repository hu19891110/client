// @flow
import UpdateEmail from './index'
import {navigateUp} from '../../actions/route-tree'
import {onChangeNewEmail, onSubmitNewEmail} from '../../actions/settings'
import {TypedConnector} from '../../util/typed-connect'

const connector = new TypedConnector()

export default connector.connect((state, dispatch, ownProps) => {
  const {waitingForResponse} = state.settings
  const {emails, error} = state.settings.email
  let email = ''
  let isVerified = false
  if (emails.length > 0) {
    email = emails[0].email
    isVerified = emails[0].isVerified
  }
  return {
    email,
    isVerified,
    error,
    waitingForResponse,
    onBack: () => {
      dispatch(navigateUp())
    },
    onSave: email => {
      dispatch(onChangeNewEmail(email))
      dispatch(onSubmitNewEmail())
    },
  }
})(UpdateEmail)
